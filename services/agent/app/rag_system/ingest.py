import json
import re
from datetime import UTC, datetime
from pathlib import Path

import pdfplumber
from langchain_core.documents import Document
from langchain_text_splitters import RecursiveCharacterTextSplitter

from .config import (
    CHUNK_OVERLAP,
    CHUNK_SIZE,
    DATA_DIR,
    FILTER_CHUNK,
    FILTER_COMPANY_ID,
    FILTER_PAGE,
    FILTER_PUBLISHED_AT,
    FILTER_SECTOR,
    FILTER_SOURCE,
    state_path,
)

# Try to import OCR libraries
try:
    import pymupdf
    import pytesseract
    from PIL import Image

    OCR_AVAILABLE = True
except ImportError:
    OCR_AVAILABLE = False


# Known company patterns for auto-extraction from PDF text
COMPANY_PATTERNS = {
    "magnit": [r"магнит", r"magnit"],
    "x5": [r"x5\s*group", r"пятерочка", r"перекресток", r"чижик"],
    "nlmk": [r"нлмк", r"nlmk"],
    "severstal": [r"северсталь", r"severstal"],
    "evraz": [r"эвраз", r"evraz"],
    "mmk": [r"ммк", r"ммаг", r"mmk"],
    "gazprom": [r"газпром", r"gazprom"],
    "lukoil": [r"лукойл", r"lukoil"],
    "rosneft": [r"роснефть", r"rosneft"],
    "tatneft": [r"татнефть", r"tatneft"],
    "sber": [r"сбер", r"sber"],
    "vtb": [r"втб", r"vtb"],
    "alfabank": [r"альфа-банк", r"альфа банк", r"alfa-bank", r"alfa bank"],
    "tbank": [r"т-банк", r"тбанк", r"t-bank", r"tbank"],
    "yandex": [r"яндекс", r"yandex"],
    "ozon": [r"озон", r"ozon"],
    "vk": [r"вк\s*компания", r"vk\s*company"],
    "mts": [r"мтс", r"mts"],
    "megafon": [r"мегафон", r"megafon"],
    "beeline": [r"билайн", r"beeline"],
    "rostec": [r"ростех", r"rostec"],
    "rosatom": [r"росатом", r"rosatom"],
    "norilsk": [r"норильский никель", r"nornickel", r"норникель"],
    "polymetal": [r"полиметалл", r"polymetal"],
    "polyus": [r"полюс", r"polyus"],
    "seligdar": [r"селигдар", r"seligdar"],
}

SECTOR_PATTERNS = {
    "retail": [r"ритейл", r"розничн", r"retail", r"супермаркет", r"гипермаркет"],
    "metallurgy": [r"металлург", r"сталь", r"steel", r"металл", r"руд"],
    "oil_gas": [r"нефть", r"газ", r"oil", r"gas", r"нефтегаз"],
    "banking": [r"банк", r"banking", r"финанс", r"кредит"],
    "it": [r"ит", r"it\b", r"технолог", r"софт", r"software", r"cyber"],
    "telecom": [r"телеком", r"связь", r"mobile", r"telecom"],
    "agriculture": [r"сельхоз", r"агро", r"agricultur", r"фермер", r"зерно"],
    "mining": [r"добыч", r"mining", r"шахт", r"месторожден"],
    "energy": [r"энергетик", r"energy", r"электростанц"],
    "chemical": [r"химическ", r"chemical", r"удобрен"],
    "transport": [r"транспорт", r"логистик", r"перевоз", r"железнодор"],
    "construction": [r"строитель", r"construction", r"девелопер"],
    "real_estate": [r"недвижимость", r"real estate", r"жилье"],
    "pharma": [r"фарма", r"pharma", r"лекарств", r"аптечн"],
    "auto": [r"авто", r"auto", r"легков", r"грузов"],
}


def extract_pdf_text(pdf_path: Path) -> list[Document]:
    """Extract text from a PDF file, returning LangChain Document objects."""
    docs = []
    try:
        pdf = pdfplumber.open(str(pdf_path))
        for i, page in enumerate(pdf.pages):
            text = page.extract_text()
            if text and text.strip():
                # Check if text is mojibake (mostly replacement chars)
                if _is_mojibake(text):
                    print(f"  Page {i + 1}: Detected mojibake, attempting OCR...")
                    text = _extract_with_ocr(pdf_path, i)
                if text and text.strip():
                    # Clean up mojibake markers and normalize
                    text = text.replace("\n", " ").strip()
                    docs.append(
                        Document(
                            page_content=text,
                            metadata={FILTER_SOURCE: pdf_path.name, FILTER_PAGE: i + 1},
                        )
                    )
        pdf.close()
    except Exception as e:
        print(f"Warning: Could not extract text from {pdf_path.name}: {e}")
    return docs


def _is_mojibake(text: str) -> bool:
    """Check if text is mostly mojibake (replacement characters)."""
    if not text:
        return True
    replacement_count = text.count("\ufffd") + text.count("�")
    return replacement_count / len(text) > 0.3


def _extract_with_ocr(pdf_path: Path, page_num: int) -> str:
    """Extract text from a PDF page using OCR."""
    if not OCR_AVAILABLE:
        print("  OCR libraries not available, skipping OCR")
        return ""
    try:
        doc = pymupdf.open(str(pdf_path))
        page = doc[page_num]
        pix = page.get_pixmap(dpi=300)
        img = Image.frombytes("RGB", [pix.width, pix.height], pix.samples)
        doc.close()

        text = pytesseract.image_to_string(img, lang="rus+eng")
        return text
    except Exception as e:
        print(f"  OCR failed: {e}")
        return ""


def _extract_entities(text: str) -> dict[str, list[str]]:
    """Extract company IDs and sectors from text using pattern matching."""
    text_lower = text.lower()
    companies = set()
    sectors = set()

    for company_id, patterns in COMPANY_PATTERNS.items():
        for pattern in patterns:
            if re.search(pattern, text_lower, re.IGNORECASE):
                companies.add(company_id)
                break

    for sector_id, patterns in SECTOR_PATTERNS.items():
        for pattern in patterns:
            if re.search(pattern, text_lower, re.IGNORECASE):
                sectors.add(sector_id)
                break

    return {
        FILTER_COMPANY_ID: list(companies) if companies else ["unknown"],
        FILTER_SECTOR: list(sectors) if sectors else ["general"],
    }


def _extract_date(text: str) -> str | None:
    """Try to extract a date from text (YYYY-MM-DD or DD.MM.YYYY)."""
    # ISO format
    match = re.search(r"\b(\d{4}-\d{2}-\d{2})\b", text)
    if match:
        return match.group(1)
    # DD.MM.YYYY
    match = re.search(r"\b(\d{2}\.\d{2}\.\d{4})\b", text)
    if match:
        d, m, y = match.group(1).split(".")
        return f"{y}-{m}-{d}"
    return None


def chunk_documents(docs: list[Document]) -> list[Document]:
    """Split documents into chunks for embedding, enriching with entity metadata."""
    splitter = RecursiveCharacterTextSplitter(
        chunk_size=CHUNK_SIZE, chunk_overlap=CHUNK_OVERLAP, separators=["\n\n", "\n", " ", ""]
    )
    chunks = []
    for doc in docs:
        chunked = splitter.split_text(doc.page_content)
        entities = _extract_entities(doc.page_content)
        pub_date = _extract_date(doc.page_content)
        for j, chunk in enumerate(chunked):
            if chunk.strip():
                metadata = {
                    **doc.metadata,
                    FILTER_CHUNK: j,
                    FILTER_COMPANY_ID: entities[FILTER_COMPANY_ID],
                    FILTER_SECTOR: entities[FILTER_SECTOR],
                }
                if pub_date:
                    metadata[FILTER_PUBLISHED_AT] = pub_date
                chunks.append(Document(page_content=chunk, metadata=metadata))
    return chunks


def load_state(index_dir: str | Path | None = None) -> dict:
    """Load the processing state file."""
    path = state_path(index_dir)
    if path.exists():
        with open(path, encoding="utf-8") as f:
            return json.load(f)
    return {"processed_files": {}}


def save_state(state: dict, index_dir: str | Path | None = None) -> None:
    """Save the processing state file."""
    path = state_path(index_dir)
    path.parent.mkdir(parents=True, exist_ok=True)
    with open(path, "w", encoding="utf-8") as f:
        json.dump(state, f, ensure_ascii=False, indent=2)


def get_new_files(state: dict, data_dir: str | Path | None = None) -> list[Path]:
    """Get list of PDF files that haven't been processed yet."""
    data_path = Path(data_dir or DATA_DIR)
    pdf_files = sorted([*data_path.glob("*.pdf"), *data_path.glob("*.PDF")])
    processed = set(state.get("processed_files", {}).keys())
    new_files = [f for f in pdf_files if f.name not in processed]
    return new_files


def update_state_add_file(state: dict, pdf_name: str, pages: int) -> None:
    """Add a file to the processing state."""
    state.setdefault("processed_files", {})[pdf_name] = {
        "pages": pages,
        "processed_at": datetime.now(UTC).isoformat(),
    }


def ingest_all(data_dir: str | Path | None = None, index_dir: str | Path | None = None) -> dict:
    """Full ingestion pipeline: extract, chunk, and return all documents."""
    state = load_state(index_dir)
    all_chunks = []
    stats = {"new_files": 0, "total_chunks": 0, "skipped": 0}

    new_files = get_new_files(state, data_dir)
    stats["new_files"] = len(new_files)

    for pdf_path in new_files:
        docs = extract_pdf_text(pdf_path)
        if docs:
            chunks = chunk_documents(docs)
            all_chunks.extend(chunks)
            update_state_add_file(state, pdf_path.name, len(docs))
            stats["total_chunks"] += len(chunks)
        else:
            stats["skipped"] += 1

    save_state(state, index_dir)
    return {"chunks": all_chunks, "stats": stats}


def ingest_pdfs_from_dir(data_dir: str | Path, index_dir: str | Path | None = None) -> dict:
    """Ingest PDFs from a specific directory."""
    return ingest_all(data_dir, index_dir)

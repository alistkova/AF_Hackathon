import os
import json
from pathlib import Path
from datetime import datetime

import pdfplumber
from langchain_text_splitters import RecursiveCharacterTextSplitter
from langchain_core.documents import Document

from .config import (
    DATA_DIR,
    INDEX_DIR,
    STATE_FILE,
    CHUNK_SIZE,
    CHUNK_OVERLAP,
    TOP_K,
    SCORE_THRESHOLD,
)


# Try to import OCR libraries
try:
    import pytesseract
    from PIL import Image
    import pymupdf

    OCR_AVAILABLE = True
except ImportError:
    OCR_AVAILABLE = False


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
                            page_content=text, metadata={"source": pdf_path.name, "page": i + 1}
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
    # Count replacement characters
    replacement_count = text.count("\ufffd") + text.count("�")
    # If more than 30% are replacement chars, it's mojibake
    return replacement_count / len(text) > 0.3


def _extract_with_ocr(pdf_path: Path, page_num: int) -> str:
    """Extract text from a PDF page using OCR."""
    if not OCR_AVAILABLE:
        print("  OCR libraries not available, skipping OCR")
        return ""
    try:
        doc = pymupdf.open(str(pdf_path))
        page = doc[page_num]
        # Render page as image at high DPI for better OCR
        pix = page.get_pixmap(dpi=300)
        img = Image.frombytes("RGB", [pix.width, pix.height], pix.samples)
        doc.close()

        # Run OCR with Russian and English
        text = pytesseract.image_to_string(img, lang="rus+eng")
        return text
    except Exception as e:
        print(f"  OCR failed: {e}")
        return ""


def chunk_documents(docs: list[Document]) -> list[Document]:
    """Split documents into chunks for embedding."""
    splitter = RecursiveCharacterTextSplitter(
        chunk_size=CHUNK_SIZE, chunk_overlap=CHUNK_OVERLAP, separators=["\n\n", "\n", " ", ""]
    )
    chunks = []
    for doc in docs:
        chunked = splitter.split_text(doc.page_content)
        for j, chunk in enumerate(chunked):
            if chunk.strip():
                chunks.append(Document(page_content=chunk, metadata={**doc.metadata, "chunk": j}))
    return chunks


def load_state() -> dict:
    """Load the processing state file."""
    if STATE_FILE.exists():
        with open(STATE_FILE, "r", encoding="utf-8") as f:
            return json.load(f)
    return {"processed_files": {}}


def save_state(state: dict):
    """Save the processing state file."""
    with open(STATE_FILE, "w", encoding="utf-8") as f:
        json.dump(state, f, ensure_ascii=False, indent=2)


def get_new_files(state: dict) -> list[Path]:
    """Get list of PDF files that haven't been processed yet."""
    pdf_files = list(DATA_DIR.glob("*.pdf"))
    processed = set(state.get("processed_files", {}).keys())
    new_files = [f for f in pdf_files if f.name not in processed]
    return new_files


def update_state_add_file(state: dict, pdf_name: str, pages: int):
    """Add a file to the processing state."""
    state.setdefault("processed_files", {})[pdf_name] = {
        "pages": pages,
        "processed_at": datetime.utcnow().isoformat(),
    }
    save_state(state)


def ingest_all() -> dict:
    """Full ingestion pipeline: extract, chunk, and return all documents."""
    state = load_state()
    all_chunks = []
    stats = {"new_files": 0, "total_chunks": 0, "skipped": 0}

    new_files = get_new_files(state)
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

    save_state(state)
    return {"chunks": all_chunks, "stats": stats}


def ingest_pdfs_from_dir(data_dir: str) -> dict:
    """Ingest PDFs from a specific directory."""
    from pathlib import Path

    global DATA_DIR
    old_data_dir = DATA_DIR
    DATA_DIR = Path(data_dir)
    try:
        return ingest_all()
    finally:
        DATA_DIR = old_data_dir

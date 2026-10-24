import os
from pathlib import Path

from app.config import get_settings

_settings = get_settings()

DATA_DIR = Path(_settings.rag_data_dir)
INDEX_DIR = Path(_settings.rag_index_dir)

CHUNK_SIZE = 1000
CHUNK_OVERLAP = 200
EMBEDDING_MODEL = "sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2"
CONTENT_URL = _settings.content_url.rstrip("/")

OLLAMA_BASE_URL = os.getenv("OLLAMA_BASE_URL") or "http://localhost:11434"
DEFAULT_MODEL = os.getenv("OLLAMA_MODEL") or "qwen"
OLLAMA_MODELS = {
    "gpt-6-astra": "gpt-6-astra:9b",
    "qwen": "qwen2.5:32b",
    "deepseek": "deepseek-v2:16b",
    "qwen_vl": "qwen2-vl:7b",
    "kimi": "moonshot-v1-8k",
}

TOP_K = 5

FILTER_COMPANY_ID = "company_id"
FILTER_SECTOR = "sector"
FILTER_PUBLISHED_AT = "published_at"
FILTER_PUBLISHED_AFTER = "published_after"
FILTER_PUBLISHED_BEFORE = "published_before"
FILTER_SOURCE = "source"
FILTER_PAGE = "page"
FILTER_CHUNK = "chunk"


def state_path(index_dir: str | Path | None = None) -> Path:
    return Path(index_dir or INDEX_DIR) / "processed_files.json"

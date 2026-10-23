import os
from pathlib import Path

from app.config import get_settings

_settings = get_settings()

DATA_DIR = Path(_settings.rag_data_dir)
INDEX_DIR = Path(_settings.rag_index_dir)

CHUNK_SIZE = 1000
CHUNK_OVERLAP = 200
EMBEDDING_MODEL = "sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2"
VECTOR_STORE_DIRNAME = "faiss_index_ascii"

OLLAMA_BASE_URL = os.getenv("OLLAMA_BASE_URL", "http://localhost:11434")
OLLAMA_MODELS = {
    "gpt-6-astra": "gpt-6-astra:9b",
    "qwen": "qwen2.5:32b",
    "deepseek": "deepseek-v2:16b",
    "qwen_vl": "qwen2-vl:7b",
    "kimi": "moonshot-v1-8k",
}

TOP_K = 5
SCORE_THRESHOLD = 0.0

# Database URL for pgvector (shared with content service)
PGVECTOR_DSN = os.getenv(
    "CONTENT_DATABASE_URL", "postgresql://content_svc:content@localhost:5432/content_db"
)
PGVECTOR_TABLE = "rag_chunks"


def vector_store_path(index_dir: str | Path | None = None) -> Path:
    return Path(index_dir or INDEX_DIR) / VECTOR_STORE_DIRNAME


def state_path(index_dir: str | Path | None = None) -> Path:
    return Path(index_dir or INDEX_DIR) / "processed_files.json"


# Filter field names (must match pgvector table columns)
FILTER_COMPANY_ID = "company_id"
FILTER_SECTOR = "sector"
FILTER_PUBLISHED_AT = "published_at"
FILTER_SOURCE = "source"
FILTER_PAGE = "page"
FILTER_CHUNK = "chunk"

import os
from pathlib import Path

# Base paths
BASE_DIR = Path(__file__).parent.parent
# Use ASCII-only paths to avoid filesystem encoding issues
DATA_DIR = BASE_DIR / "data"
INDEX_DIR = Path("C:/temp/alfa_rag_index")
CACHE_DIR = Path("C:/temp/alfa_rag_cache")

# Create directories
for d in [DATA_DIR, INDEX_DIR, CACHE_DIR]:
    d.mkdir(parents=True, exist_ok=True)

# PDF processing
CHUNK_SIZE = 1000
CHUNK_OVERLAP = 200

# Embedding model (Russian-friendly)
# Use a smaller model that's more likely to be cached
# Use TF-IDF for no-download embeddings (fallback)
EMBEDDING_MODEL = "tfidf"

# Vector store
VECTOR_STORE_PATH = str(INDEX_DIR / "faiss_index_ascii")

# Ollama settings (for local LLMs)
OLLAMA_BASE_URL = os.getenv("OLLAMA_BASE_URL", "http://localhost:11434")
OLLAMA_MODELS = {
    "gemma": "gemma2:27b",
    "gpt-5.6-sol": "gpt-5.6-sol:9b",
    "qwen": "qwen2.5:32b",
    "deepseek": "deepseek-v2:16b",
    "qwen_vl": "qwen2-vl:7b",
    "kimi": "moonshot-v1-8k",
}

# Retrieval settings
TOP_K = 5
SCORE_THRESHOLD = 0.5

# Incremental update settings
STATE_FILE = INDEX_DIR / "processed_files.json"

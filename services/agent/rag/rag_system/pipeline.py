"""Main RAG pipeline orchestrating ingestion, indexing, and querying."""

from typing import Dict, Any, List
from pathlib import Path

from .ingest import ingest_all, load_state
from .embeddings import (
    create_vector_store,
    save_vector_store,
    load_vector_store,
    add_chunks_to_store,
)
from .retrieve import retrieve, retrieve_with_context, Retriever
from .generate import rag_query, rag_stream, generate_with_ollama
from .config import VECTOR_STORE_PATH


def build_index(force_rebuild: bool = False) -> Dict[str, Any]:
    """Build or rebuild the vector index from all PDFs in data directory."""
    result = ingest_all()
    chunks = result["chunks"]
    stats = result["stats"]

    if not chunks and not force_rebuild:
        # Try loading existing index
        existing = load_vector_store()
        if existing:
            return {
                "status": "unchanged",
                "message": "No new files, existing index loaded",
                "stats": stats,
            }
        return {"status": "empty", "message": "No documents to index", "stats": stats}

    # Create new vector store
    vector_store = create_vector_store(chunks)
    save_vector_store(vector_store)

    return {
        "status": "rebuilt" if force_rebuild else "built",
        "message": f"Indexed {stats['total_chunks']} chunks from {stats['new_files']} new files",
        "stats": stats,
    }


def incremental_update() -> Dict[str, Any]:
    """Incrementally add new PDFs to existing index."""
    result = ingest_all()
    chunks = result["chunks"]
    stats = result["stats"]

    if not chunks:
        return {"status": "no_changes", "message": "No new files to process", "stats": stats}

    # Load or create vector store
    vector_store = load_vector_store()
    if vector_store is None:
        vector_store = create_vector_store(chunks)
    else:
        add_chunks_to_store(vector_store, chunks)

    save_vector_store(vector_store)

    return {
        "status": "updated",
        "message": f"Added {stats['total_chunks']} chunks from {stats['new_files']} new files",
        "stats": stats,
    }


def query(
    question: str, model: str = "qwen", k: int = 5, temperature: float = 0.3, stream: bool = False
) -> Dict[str, Any] | List[str]:
    """Query the RAG system."""
    if stream:
        return list(rag_stream(question, model, k, temperature))
    return rag_query(question, model, k, temperature)


def get_retriever(k: int = 5, score_threshold: float = 0.5) -> Retriever:
    """Get a retriever instance for custom use."""
    return Retriever(k=k, score_threshold=score_threshold)


def get_stats() -> Dict[str, Any]:
    """Get system statistics."""
    state = load_state()
    processed = state.get("processed_files", {})
    total_pages = sum(f.get("pages", 0) for f in processed.values())

    index_exists = Path(VECTOR_STORE_PATH).exists()

    return {
        "processed_files": len(processed),
        "total_pages": total_pages,
        "files": processed,
        "index_exists": index_exists,
    }

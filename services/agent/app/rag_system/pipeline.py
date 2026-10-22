"""Main RAG pipeline orchestrating ingestion, indexing, and querying."""

from pathlib import Path
from typing import Any

from .config import DATA_DIR, INDEX_DIR, SCORE_THRESHOLD, vector_store_path
from .embeddings import (
    create_vector_store,
    load_vector_store,
    save_vector_store,
)
from .generate import rag_query, rag_stream
from .ingest import ingest_pdfs_from_dir, load_state
from .retrieve import Retriever


def build_index(
    data_dir: str | Path = DATA_DIR,
    index_dir: str | Path = INDEX_DIR,
    force_rebuild: bool = False,
) -> dict[str, Any]:
    """Build or rebuild the vector index from all PDFs in data directory."""
    result = ingest_pdfs_from_dir(data_dir, index_dir)
    chunks = result["chunks"]
    stats = result["stats"]

    if not chunks:
        if not force_rebuild:
            existing = load_vector_store(index_dir)
            if existing:
                return {
                    "status": "unchanged",
                    "message": "No new files, existing index loaded",
                    "stats": stats,
                }
        return {"status": "empty", "message": "No documents to index", "stats": stats}

    vector_store = create_vector_store(chunks, index_dir)
    save_vector_store(vector_store, index_dir)

    return {
        "status": "rebuilt" if force_rebuild else "built",
        "message": f"Indexed {stats['total_chunks']} chunks from {stats['new_files']} new files",
        "stats": stats,
    }


def update_index(
    data_dir: str | Path = DATA_DIR,
    index_dir: str | Path = INDEX_DIR,
) -> dict[str, Any]:
    """Incrementally add new PDFs to existing index."""
    result = ingest_pdfs_from_dir(data_dir, index_dir)
    chunks = result["chunks"]
    stats = result["stats"]

    if not chunks:
        return {"status": "no_changes", "message": "No new files to process", "stats": stats}

    vector_store = load_vector_store(index_dir)
    if vector_store is None:
        vector_store = create_vector_store(chunks, index_dir)
    else:
        existing_docs = list(vector_store.docstore._dict.values())
        vector_store = create_vector_store([*existing_docs, *chunks], index_dir)

    save_vector_store(vector_store, index_dir)

    return {
        "status": "updated",
        "message": f"Added {stats['total_chunks']} chunks from {stats['new_files']} new files",
        "stats": stats,
    }


def query_index(
    question: str,
    index_dir: str | Path = INDEX_DIR,
    model: str = "gpt-5.6-sol",
    k: int = 5,
    temperature: float = 0.3,
    stream: bool = False,
) -> dict[str, Any] | list[str]:
    """Query the RAG system."""
    if stream:
        return list(
            rag_stream(
                question,
                index_dir=index_dir,
                model=model,
                k=k,
                temperature=temperature,
            )
        )
    return rag_query(
        question,
        index_dir=index_dir,
        model=model,
        k=k,
        temperature=temperature,
    )


def index_stats(index_dir: str | Path = INDEX_DIR) -> dict[str, Any]:
    """Get system statistics."""
    state = load_state(index_dir)
    processed = state.get("processed_files", {})
    total_pages = sum(f.get("pages", 0) for f in processed.values())
    return {
        "processed_files": len(processed),
        "total_pages": total_pages,
        "files": processed,
        "index_dir": str(index_dir),
        "index_exists": vector_store_path(index_dir).exists(),
    }


def get_retriever(
    index_dir: str | Path = INDEX_DIR,
    k: int = 5,
    score_threshold: float = SCORE_THRESHOLD,
) -> Retriever:
    """Get a retriever instance for custom use."""
    return Retriever(index_dir=index_dir, k=k, score_threshold=score_threshold)

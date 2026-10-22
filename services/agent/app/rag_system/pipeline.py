"""Main RAG pipeline orchestrating ingestion, indexing, and querying."""

from typing import Dict, Any, List
from pathlib import Path

from .ingest import ingest_all, load_state, ingest_pdfs_from_dir
from .embeddings import (
    create_vector_store,
    save_vector_store,
    load_vector_store,
    add_chunks_to_store,
)
from .retrieve import retrieve, retrieve_with_context, Retriever
from .generate import rag_query, rag_stream, generate_with_ollama


def build_index(
    data_dir: str = "services/agent/rag/data",
    index_dir: str = "C:/temp/alfa_rag_index",
    force_rebuild: bool = False,
) -> Dict[str, Any]:
    """Build or rebuild the vector index from all PDFs in data directory."""
    import os

    os.environ["RAG_INDEX_DIR"] = index_dir
    os.environ["RAG_DATA_DIR"] = data_dir
    result = ingest_pdfs_from_dir(data_dir)
    chunks = result["chunks"]
    stats = result["stats"]

    if not chunks and not force_rebuild:
        existing = load_vector_store(index_dir)
        if existing:
            return {
                "status": "unchanged",
                "message": "No new files, existing index loaded",
                "stats": stats,
            }
        return {"status": "empty", "message": "No documents to index", "stats": stats}

    vector_store = create_vector_store(chunks)
    save_vector_store(vector_store, index_dir)

    return {
        "status": "rebuilt" if force_rebuild else "built",
        "message": f"Indexed {stats['total_chunks']} chunks from {stats['new_files']} new files",
        "stats": stats,
    }


def update_index(
    data_dir: str = "services/agent/rag/data", index_dir: str = "C:/temp/alfa_rag_index"
) -> Dict[str, Any]:
    """Incrementally add new PDFs to existing index."""
    import os

    os.environ["RAG_INDEX_DIR"] = index_dir
    os.environ["RAG_DATA_DIR"] = data_dir
    result = ingest_pdfs_from_dir(data_dir)
    chunks = result["chunks"]
    stats = result["stats"]

    if not chunks:
        return {"status": "no_changes", "message": "No new files to process", "stats": stats}

    vector_store = load_vector_store(index_dir)
    if vector_store is None:
        vector_store = create_vector_store(chunks)
    else:
        add_chunks_to_store(vector_store, chunks)

    save_vector_store(vector_store, index_dir)

    return {
        "status": "updated",
        "message": f"Added {stats['total_chunks']} chunks from {stats['new_files']} new files",
        "stats": stats,
    }


def query_index(
    question: str,
    index_dir: str = "C:/temp/alfa_rag_index",
    model: str = "gpt-5.6-sol",
    k: int = 5,
    temperature: float = 0.3,
    stream: bool = False,
) -> Dict[str, Any] | List[str]:
    """Query the RAG system."""
    import os

    os.environ["RAG_INDEX_DIR"] = index_dir
    from .generate import rag_query, rag_stream

    if stream:
        return list(rag_stream(question, model, k, temperature))
    return rag_query(question, model, k, temperature)


def index_stats(index_dir: str = "C:/temp/alfa_rag_index") -> Dict[str, Any]:
    """Get system statistics."""
    import os

    os.environ["RAG_INDEX_DIR"] = index_dir
    from .config import VECTOR_STORE_PATH

    state = load_state()
    processed = state.get("processed_files", {})
    total_pages = sum(f.get("pages", 0) for f in processed.values())
    index_exists = Path(index_dir).exists()
    return {
        "processed_files": len(processed),
        "total_pages": total_pages,
        "files": processed,
        "index_exists": index_exists,
    }


def get_retriever(
    index_dir: str = "C:/temp/alfa_rag_index", k: int = 5, score_threshold: float = 0.5
) -> Retriever:
    """Get a retriever instance for custom use."""
    import os

    os.environ["RAG_INDEX_DIR"] = index_dir
    return Retriever(k=k, score_threshold=score_threshold)

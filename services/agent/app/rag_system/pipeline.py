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
from .retrieve import Retriever, get_chunks_by_metadata, retrieve_with_filters


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
    # PGVector auto-appends, no need to rebuild

    save_vector_store(vector_store, index_dir)

    return {
        "status": "updated",
        "message": f"Added {stats['total_chunks']} chunks from {stats['new_files']} new files",
        "stats": stats,
    }


def query_index(
    question: str,
    index_dir: str | Path = INDEX_DIR,
    model: str = "qwen",
    k: int = 5,
    temperature: float = 0.3,
    stream: bool = False,
    company_id: str | None = None,
    sector: str | None = None,
    published_after: str | None = None,
    published_before: str | None = None,
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
                company_id=company_id,
                sector=sector,
                published_after=published_after,
                published_before=published_before,
            )
        )
    return rag_query(
        question,
        index_dir=index_dir,
        model=model,
        k=k,
        temperature=temperature,
        company_id=company_id,
        sector=sector,
        published_after=published_after,
        published_before=published_before,
    )


def search_chunks(
    query: str,
    index_dir: str | Path = INDEX_DIR,
    k: int = 10,
    company_id: str | None = None,
    sector: str | None = None,
    published_after: str | None = None,
    published_before: str | None = None,
) -> dict[str, Any]:
    """Search chunks with filters (for agent/API use) - returns chunks WITHOUT LLM generation."""
    from .retrieve import retrieve_with_filters, source_labels

    docs = retrieve_with_filters(
        query,
        index_dir=index_dir,
        k=k,
        company_id=company_id,
        sector=sector,
        published_after=published_after,
        published_before=published_before,
    )
    return {
        "query": query,
        "chunks": [
            {
                "content": d.page_content,
                "source": d.metadata.get("source"),
                "page": d.metadata.get("page"),
                "chunk": d.metadata.get("chunk"),
                "company_ids": d.metadata.get("company_id"),
                "sectors": d.metadata.get("sector"),
                "published_at": d.metadata.get("published_at"),
            }
            for d in docs
        ],
        "sources": source_labels(docs),
    }


def list_chunks(
    index_dir: str | Path = INDEX_DIR,
    company_id: str | None = None,
    sector: str | None = None,
    published_after: str | None = None,
    published_before: str | None = None,
    k: int = 100,
) -> dict[str, Any]:
    """List chunks by metadata only (no semantic query)."""
    from .retrieve import get_chunks_by_metadata, source_labels

    docs = get_chunks_by_metadata(
        index_dir=index_dir,
        company_id=company_id,
        sector=sector,
        published_after=published_after,
        published_before=published_before,
        k=k,
    )
    return {
        "chunks": [
            {
                "content": d.page_content,
                "source": d.metadata.get("source"),
                "page": d.metadata.get("page"),
                "chunk": d.metadata.get("chunk"),
                "company_ids": d.metadata.get("company_id"),
                "sectors": d.metadata.get("sector"),
                "published_at": d.metadata.get("published_at"),
            }
            for d in docs
        ],
        "sources": source_labels(docs),
    }


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

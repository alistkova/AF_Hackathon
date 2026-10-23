"""Main RAG pipeline orchestrating ingestion, indexing, and querying."""

from pathlib import Path
from typing import Any

from langchain_core.documents import Document

from .config import DATA_DIR, DEFAULT_MODEL, INDEX_DIR
from .embeddings import add_chunks, create_vector_store
from .generate import rag_query, rag_stream
from .ingest import ingest_pdfs_from_dir, load_state, reset_state
from .retrieve import Retriever, get_chunks_by_metadata, retrieve_with_filters, source_labels
from .store import count_chunks, drop_chunks


def build_index(
    data_dir: str | Path = DATA_DIR,
    index_dir: str | Path = INDEX_DIR,
    force_rebuild: bool = False,
) -> dict[str, Any]:
    """Build or rebuild the vector index from all PDFs in data directory."""
    if force_rebuild:
        reset_state(index_dir)
        drop_chunks()

    result = ingest_pdfs_from_dir(data_dir, index_dir)
    chunks = result["chunks"]
    stats = result["stats"]

    if chunks:
        create_vector_store(chunks)
        return {
            "status": "rebuilt" if force_rebuild else "built",
            "message": (
                f"Indexed {stats['total_chunks']} chunks from {stats['new_files']} new files"
            ),
            "stats": stats,
        }

    indexed = count_chunks()
    if indexed:
        return {
            "status": "unchanged",
            "message": f"No new files, {indexed} chunks already indexed",
            "stats": stats,
        }
    return {"status": "empty", "message": "No documents to index", "stats": stats}


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

    add_chunks(chunks)

    return {
        "status": "updated",
        "message": f"Added {stats['total_chunks']} chunks from {stats['new_files']} new files",
        "stats": stats,
    }


def query_index(
    question: str,
    model: str = DEFAULT_MODEL,
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
    k: int = 10,
    company_id: str | None = None,
    sector: str | None = None,
    published_after: str | None = None,
    published_before: str | None = None,
) -> dict[str, Any]:
    """Search chunks with filters (for agent/API use) - returns chunks WITHOUT LLM generation."""
    docs = retrieve_with_filters(
        query,
        k=k,
        company_id=company_id,
        sector=sector,
        published_after=published_after,
        published_before=published_before,
    )
    return {
        "query": query,
        "chunks": [_chunk_payload(doc) for doc in docs],
        "sources": source_labels(docs),
    }


def list_chunks(
    company_id: str | None = None,
    sector: str | None = None,
    published_after: str | None = None,
    published_before: str | None = None,
    k: int = 100,
) -> dict[str, Any]:
    """List chunks by metadata only (no semantic query)."""
    docs = get_chunks_by_metadata(
        company_id=company_id,
        sector=sector,
        published_after=published_after,
        published_before=published_before,
        k=k,
    )
    return {
        "chunks": [_chunk_payload(doc) for doc in docs],
        "sources": source_labels(docs),
    }


def _chunk_payload(doc: Document) -> dict[str, Any]:
    return {
        "content": doc.page_content,
        "source": doc.metadata.get("source"),
        "page": doc.metadata.get("page"),
        "chunk": doc.metadata.get("chunk"),
        "company_id": doc.metadata.get("company_id"),
        "sector": doc.metadata.get("sector"),
        "published_at": doc.metadata.get("published_at"),
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
        "index_exists": count_chunks() > 0,
    }


def get_retriever(
    k: int = 5,
    max_distance: float | None = None,
) -> Retriever:
    """Get a retriever instance for custom use."""
    return Retriever(k=k, max_distance=max_distance)

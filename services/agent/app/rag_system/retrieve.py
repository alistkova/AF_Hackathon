from pathlib import Path

from langchain_core.documents import Document

from .config import (
    FILTER_COMPANY_ID,
    FILTER_PUBLISHED_AT,
    FILTER_SECTOR,
    SCORE_THRESHOLD,
    TOP_K,
)
from .embeddings import load_vector_store
from .embeddings import search as search_embeddings, search_by_metadata


def format_context(docs: list[Document]) -> str:
    """Format retrieved documents as a context string for the LLM."""
    if not docs:
        return "No relevant information found."
    context_parts = []
    for i, doc in enumerate(docs, 1):
        source = doc.metadata.get("source", "unknown")
        page = doc.metadata.get("page", "?")
        chunk = doc.metadata.get("chunk", "?")
        context_parts.append(
            f"[Source {i}: {source}, page {page}, chunk {chunk}]\n{doc.page_content}"
        )
    return "\n\n---\n\n".join(context_parts)


def source_labels(docs: list[Document]) -> list[str]:
    """Return human-readable source labels for retrieved documents."""
    return [
        f"{doc.metadata.get('source', 'unknown')} (page {doc.metadata.get('page', '?')})"
        for doc in docs
    ]


def retrieve(
    query: str,
    index_dir: str | Path | None = None,
    k: int = TOP_K,
    score_threshold: float = SCORE_THRESHOLD,
) -> list[Document]:
    """Retrieve relevant documents for a query (no filters - semantic search only)."""
    vector_store = load_vector_store(index_dir)
    if vector_store is None:
        return []
    return search_embeddings(vector_store, query, k=k, score_threshold=score_threshold)


def retrieve_with_filters(
    query: str,
    index_dir: str | Path | None = None,
    k: int = TOP_K,
    score_threshold: float = SCORE_THRESHOLD,
    company_id: str | None = None,
    sector: str | None = None,
    published_after: str | None = None,
    published_before: str | None = None,
) -> list[Document]:
    """Retrieve relevant documents with metadata filters (company, sector, date range)."""
    vector_store = load_vector_store(index_dir)
    if vector_store is None:
        return []

    # Build metadata filter for pgvector
    filter_dict = {}
    if company_id:
        filter_dict[FILTER_COMPANY_ID] = {"$contains": company_id}
    if sector:
        filter_dict[FILTER_SECTOR] = {"$contains": sector}
    if published_after:
        filter_dict[FILTER_PUBLISHED_AT] = {"$gte": published_after}
    if published_before:
        if FILTER_PUBLISHED_AT in filter_dict:
            filter_dict[FILTER_PUBLISHED_AT]["$lte"] = published_before
        else:
            filter_dict[FILTER_PUBLISHED_AT] = {"$lte": published_before}

    if filter_dict:
        return search_embeddings(
            vector_store, query, k=k, score_threshold=score_threshold, filter=filter_dict
        )
    else:
        return search_embeddings(vector_store, query, k=k, score_threshold=score_threshold)


def get_chunks_by_metadata(
    index_dir: str | Path | None = None,
    company_id: str | None = None,
    sector: str | None = None,
    published_after: str | None = None,
    published_before: str | None = None,
    k: int = 100,
) -> list[Document]:
    """Get chunks by metadata only (no semantic query), useful for listing news for a company."""
    vector_store = load_vector_store(index_dir)
    if vector_store is None:
        return []

    filter_dict = {}
    if company_id:
        filter_dict[FILTER_COMPANY_ID] = {"$contains": company_id}
    if sector:
        filter_dict[FILTER_SECTOR] = {"$contains": sector}
    if published_after:
        filter_dict[FILTER_PUBLISHED_AT] = {"$gte": published_after}
    if published_before:
        if FILTER_PUBLISHED_AT in filter_dict:
            filter_dict[FILTER_PUBLISHED_AT]["$lte"] = published_before
        else:
            filter_dict[FILTER_PUBLISHED_AT] = {"$lte": published_before}

    if not filter_dict:
        return []

    return search_by_metadata(vector_store, filter_dict, k=k)


def retrieve_with_context(
    query: str,
    index_dir: str | Path | None = None,
    k: int = TOP_K,
    score_threshold: float = SCORE_THRESHOLD,
) -> str:
    """Retrieve and format as context string for LLM."""
    return format_context(retrieve(query, index_dir, k, score_threshold))


class Retriever:
    """Simple retriever class for integration."""

    def __init__(
        self,
        index_dir: str | Path | None = None,
        k: int = TOP_K,
        score_threshold: float = SCORE_THRESHOLD,
    ):
        self.index_dir = index_dir
        self.k = k
        self.score_threshold = score_threshold
        self._vector_store = None

    @property
    def vector_store(self):
        if self._vector_store is None:
            self._vector_store = load_vector_store(self.index_dir)
        return self._vector_store

    def invoke(self, query: str) -> list[Document]:
        if self.vector_store is None:
            return []
        return search_embeddings(
            self.vector_store, query, k=self.k, score_threshold=self.score_threshold
        )

    def invoke_with_filters(
        self,
        query: str,
        company_id: str | None = None,
        sector: str | None = None,
        published_after: str | None = None,
        published_before: str | None = None,
    ) -> list[Document:
        """Retrieve with metadata filters."""
        if self.vector_store is None:
            return []
        return retrieve_with_filters(
            query,
            index_dir=self.index_dir,
            k=self.k,
            score_threshold=self.score_threshold,
            company_id=company_id,
            sector=sector,
            published_after=published_after,
            published_before=published_before,
        )

    def get_by_metadata(
        self,
        company_id: str | None = None,
        sector: str | None = None,
        published_after: str | None = None,
        published_before: str | None = None,
        k: int = 100,
    ) -> list[Document]:
        """Get chunks by metadata only (no semantic query)."""
        if self.vector_store is None:
            return []
        return get_chunks_by_metadata(
            index_dir=self.index_dir,
            company_id=company_id,
            sector=sector,
            published_after=published_after,
            published_before=published_before,
            k=k,
        )

    def get_context(self, query: str) -> str:
        return format_context(self.invoke(query))

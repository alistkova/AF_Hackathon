from langchain_core.documents import Document

from . import store
from .config import (
    FILTER_COMPANY_ID,
    FILTER_PUBLISHED_AFTER,
    FILTER_PUBLISHED_BEFORE,
    FILTER_SECTOR,
    TOP_K,
)
from .embeddings import embed_query


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


def build_filter(
    company_id: str | None = None,
    sector: str | None = None,
    published_after: str | None = None,
    published_before: str | None = None,
) -> dict[str, str]:
    """Build metadata filter params for the content service."""
    filters: dict[str, str] = {}
    if company_id:
        filters[FILTER_COMPANY_ID] = company_id
    if sector:
        filters[FILTER_SECTOR] = sector
    if published_after:
        filters[FILTER_PUBLISHED_AFTER] = published_after
    if published_before:
        filters[FILTER_PUBLISHED_BEFORE] = published_before
    return filters


def retrieve(
    query: str,
    k: int = TOP_K,
    max_distance: float | None = None,
) -> list[Document]:
    """Retrieve relevant documents for a query (no filters - semantic search only)."""
    return store.search(embed_query(query), k=k, max_distance=max_distance)


def retrieve_with_filters(
    query: str,
    k: int = TOP_K,
    max_distance: float | None = None,
    company_id: str | None = None,
    sector: str | None = None,
    published_after: str | None = None,
    published_before: str | None = None,
) -> list[Document]:
    """Retrieve relevant documents with metadata filters (company, sector, date range)."""
    filters = build_filter(
        company_id=company_id,
        sector=sector,
        published_after=published_after,
        published_before=published_before,
    )
    return store.search(embed_query(query), k=k, max_distance=max_distance, filters=filters or None)


def get_chunks_by_metadata(
    company_id: str | None = None,
    sector: str | None = None,
    published_after: str | None = None,
    published_before: str | None = None,
    k: int = 100,
) -> list[Document]:
    """Get chunks by metadata only (no semantic query), useful for listing news for a company."""
    filters = build_filter(
        company_id=company_id,
        sector=sector,
        published_after=published_after,
        published_before=published_before,
    )
    if not filters:
        return []
    return store.list_by_metadata(filters, k)


def retrieve_with_context(
    query: str,
    k: int = TOP_K,
    max_distance: float | None = None,
) -> str:
    """Retrieve and format as context string for LLM."""
    return format_context(retrieve(query, k, max_distance))


class Retriever:
    """Simple retriever class for integration."""

    def __init__(
        self,
        k: int = TOP_K,
        max_distance: float | None = None,
    ):
        self.k = k
        self.max_distance = max_distance

    def invoke(self, query: str) -> list[Document]:
        return retrieve(query, k=self.k, max_distance=self.max_distance)

    def invoke_with_filters(
        self,
        query: str,
        company_id: str | None = None,
        sector: str | None = None,
        published_after: str | None = None,
        published_before: str | None = None,
    ) -> list[Document]:
        """Retrieve with metadata filters."""
        return retrieve_with_filters(
            query,
            k=self.k,
            max_distance=self.max_distance,
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
        return get_chunks_by_metadata(
            company_id=company_id,
            sector=sector,
            published_after=published_after,
            published_before=published_before,
            k=k,
        )

    def get_context(self, query: str) -> str:
        return format_context(self.invoke(query))

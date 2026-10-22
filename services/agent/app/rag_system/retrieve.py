from pathlib import Path

from langchain_core.documents import Document

from .config import SCORE_THRESHOLD, TOP_K
from .embeddings import load_vector_store
from .embeddings import search as search_embeddings


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
    """Retrieve relevant documents for a query."""
    vector_store = load_vector_store(index_dir)
    if vector_store is None:
        return []
    return search_embeddings(vector_store, query, k=k, score_threshold=score_threshold)


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

    def get_context(self, query: str) -> str:
        return format_context(self.invoke(query))

import os
from typing import Optional
from langchain_community.vectorstores import FAISS
from langchain_core.documents import Document

from .config import VECTOR_STORE_PATH, TOP_K, SCORE_THRESHOLD
from .embeddings import get_embeddings, load_vector_store, search as search_embeddings


def retrieve(
    query: str, k: int = TOP_K, score_threshold: float = SCORE_THRESHOLD
) -> list[Document]:
    """Retrieve relevant documents for a query."""
    vector_store = load_vector_store()
    if vector_store is None:
        return []
    return search_embeddings(vector_store, query, k=k, score_threshold=score_threshold)


def retrieve_with_context(
    query: str, k: int = TOP_K, score_threshold: float = SCORE_THRESHOLD
) -> str:
    """Retrieve and format as context string for LLM."""
    docs = retrieve(query, k, score_threshold)
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


class Retriever:
    """Simple retriever class for integration."""

    def __init__(self, k: int = TOP_K, score_threshold: float = SCORE_THRESHOLD):
        self.k = k
        self.score_threshold = score_threshold
        self._vector_store = None

    @property
    def vector_store(self):
        if self._vector_store is None:
            self._vector_store = load_vector_store()
        return self._vector_store

    def invoke(self, query: str) -> list[Document]:
        if self.vector_store is None:
            return []
        return search_embeddings(
            self.vector_store, query, k=self.k, score_threshold=self.score_threshold
        )

    def get_context(self, query: str) -> str:
        docs = self.invoke(query)
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

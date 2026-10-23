import os
import pickle
from pathlib import Path
from typing import Any

from langchain_core.documents import Document
from langchain_core.embeddings import Embeddings
from langchain_postgres import PGVector

from .config import (
    EMBEDDING_MODEL,
    FILTER_CHUNK,
    FILTER_COMPANY_ID,
    FILTER_PAGE,
    FILTER_PUBLISHED_AT,
    FILTER_SECTOR,
    FILTER_SOURCE,
    PGVECTOR_DSN,
    PGVECTOR_TABLE,
)


class TfidfEmbeddings(Embeddings):
    """Wrapper for sklearn TfidfVectorizer to match LangChain embeddings interface."""

    def __init__(self):
        from sklearn.feature_extraction.text import TfidfVectorizer

        self.vectorizer = TfidfVectorizer(stop_words=None, min_df=1)
        self._fitted = False

    def __call__(self, text: str) -> list[float]:
        return self.embed_query(text)

    def embed_documents(self, texts: list[str]) -> list[list[float]]:
        if not self._fitted:
            self.vectorizer.fit(texts)
            self._fitted = True
        return self.vectorizer.transform(texts).toarray().tolist()

    def embed_query(self, text: str) -> list[float]:
        if not self._fitted:
            raise RuntimeError("Vectorizer not fitted. Call embed_documents first.")
        return self.vectorizer.transform([text]).toarray()[0].tolist()

    def save_vectorizer(self, path: str):
        """Save fitted vectorizer to disk."""
        with open(path, "wb") as f:
            pickle.dump(self.vectorizer, f)

    @classmethod
    def load_vectorizer(cls, path: str):
        """Load fitted vectorizer from disk."""
        with open(path, "rb") as f:
            vectorizer = pickle.load(f)
        instance = cls()
        instance.vectorizer = vectorizer
        instance._fitted = True
        return instance


_embedding_models: dict[str, Any] = {}
_pgvector_stores: dict[str, Any] = {}


def get_embeddings(index_dir: str | Path | None = None):
    """Get or create the embedding model."""
    key = str(index_dir or "default")
    if key not in _embedding_models:
        try:
            from langchain_huggingface import HuggingFaceEmbeddings
        except ImportError as exc:
            raise RuntimeError(
                "HuggingFace embeddings require the 'hf' extra: uv sync --extra hf"
            ) from exc

        _embedding_models[key] = HuggingFaceEmbeddings(
            model_name=EMBEDDING_MODEL,
            model_kwargs={"device": "cpu"},
            encode_kwargs={"normalize_embeddings": True},
        )
    return _embedding_models[key]


def _get_pgvector_store(index_dir: str | Path | None = None, embeddings=None) -> PGVector:
    """Get or create PGVector store connected to PostgreSQL."""
    key = str(index_dir or "default")
    if key not in _pgvector_stores:
        if embeddings is None:
            embeddings = get_embeddings(index_dir)
        _pgvector_stores[key] = PGVector(
            embeddings=embeddings,
            collection_name=PGVECTOR_TABLE,
            connection=PGVECTOR_DSN,
            use_jsonb=True,
        )
    return _pgvector_stores[key]


def create_vector_store(chunks: list[Document], index_dir: str | Path | None = None) -> PGVector:
    """Add chunks to PGVector store."""
    if not chunks:
        raise ValueError("Cannot create a vector store from an empty document list")

    embeddings = get_embeddings(index_dir)
    store = _get_pgvector_store(index_dir, embeddings)

    texts = [c.page_content for c in chunks]
    metadatas = []
    import json
    for c in chunks:
        md = c.metadata.copy()
        # Convert list fields to JSON strings for pgvector JSONB
        for key in [FILTER_COMPANY_ID, FILTER_SECTOR]:
            if key in md and isinstance(md[key], list):
                md[key] = json.dumps(md[key])
        metadatas.append(md)

    store.add_texts(texts=texts, metadatas=metadatas)
    return store


def load_vector_store(index_dir: str | Path | None = None) -> PGVector | None:
    """Load existing PGVector store."""
    try:
        embeddings = get_embeddings(index_dir)
        return _get_pgvector_store(index_dir, embeddings)
    except Exception:
        return None


def save_vector_store(vector_store: PGVector, index_dir: str | Path | None = None) -> None:
    """PGVector persists automatically, no-op."""
    pass


def search(
    vector_store: PGVector,
    query: str,
    k: int = 5,
    score_threshold: float = 0.0,
    filter: dict | None = None,
) -> list[Document]:
    """Search the vector store for relevant chunks with optional metadata filter."""
    if filter:
        results = vector_store.similarity_search_with_score(query, k=k, filter=filter)
    else:
        results = vector_store.similarity_search_with_score(query, k=k)
    return [doc for doc, score in results if score >= score_threshold]


def search_by_metadata(
    vector_store: PGVector,
    filter: dict,
    k: int = 100,
) -> list[Document]:
    """Search by metadata only (no vector similarity), useful for filtering."""
    return vector_store.similarity_search(query="", k=k, filter=filter)

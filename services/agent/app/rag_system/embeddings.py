from typing import Any

from langchain_core.documents import Document

from . import store
from .config import EMBEDDING_MODEL

_embedding_models: dict[str, Any] = {}


def get_embeddings(model_name: str | None = None):
    key = model_name or EMBEDDING_MODEL
    if key not in _embedding_models:
        try:
            from langchain_huggingface import HuggingFaceEmbeddings
        except ImportError as exc:
            raise RuntimeError(
                "HuggingFace embeddings require the 'hf' extra: uv sync --extra hf"
            ) from exc

        _embedding_models[key] = HuggingFaceEmbeddings(
            model_name=key,
            model_kwargs={"device": "cpu"},
            encode_kwargs={"normalize_embeddings": True},
        )
    return _embedding_models[key]


def embed_documents(texts: list[str]) -> list[list[float]]:
    return get_embeddings().embed_documents(texts)


def embed_query(text: str) -> list[float]:
    return get_embeddings().embed_query(text)


def create_vector_store(chunks: list[Document]) -> int:
    if not chunks:
        raise ValueError("Cannot create a vector store from an empty document list")
    return store.add_chunks(chunks, embed_documents([chunk.page_content for chunk in chunks]))


def add_chunks(chunks: list[Document]) -> int:
    if not chunks:
        return 0
    return store.add_chunks(chunks, embed_documents([chunk.page_content for chunk in chunks]))

from typing import Any

import httpx
from langchain_core.documents import Document

from .config import CONTENT_URL

_TIMEOUT_SECONDS = 30.0


class ContentStoreError(RuntimeError):
    def __init__(self, message: str, status_code: int | None = None) -> None:
        super().__init__(message)
        self.status_code = status_code


def _client() -> httpx.Client:
    return httpx.Client(base_url=CONTENT_URL, timeout=_TIMEOUT_SECONDS)


def _request(method: str, path: str, payload: dict[str, Any] | None = None) -> dict[str, Any]:
    try:
        with _client() as client:
            response = client.request(method, path, json=payload)
    except httpx.HTTPError as exc:
        raise ContentStoreError(f"content service is unavailable: {exc}") from exc
    if not response.is_success:
        detail = response.text[:200]
        raise ContentStoreError(
            f"content service returned {response.status_code}: {detail}",
            status_code=response.status_code,
        )
    if not response.content:
        return {}
    return response.json()


def chunk_payload(doc: Document, embedding: list[float]) -> dict[str, Any]:
    return {
        "content": doc.page_content,
        "source": doc.metadata.get("source") or "",
        "page": int(doc.metadata.get("page") or 0),
        "chunk": int(doc.metadata.get("chunk") or 0),
        "company_id": doc.metadata.get("company_id") or "",
        "sector": doc.metadata.get("sector") or "",
        "published_at": doc.metadata.get("published_at") or None,
        "embedding": embedding,
    }


def add_chunks(chunks: list[Document], vectors: list[list[float]]) -> int:
    if len(chunks) != len(vectors):
        raise ValueError("chunks and vectors must have the same length")
    if not chunks:
        return 0
    payload = {
        "chunks": [chunk_payload(doc, vector) for doc, vector in zip(chunks, vectors, strict=True)]
    }
    data = _request("POST", "/api/v1/rag/chunks", payload)
    return int(data.get("upserted", 0))


def search(
    embedding: list[float],
    k: int,
    max_distance: float | None = None,
    filters: dict[str, str] | None = None,
) -> list[Document]:
    payload: dict[str, Any] = {"embedding": embedding, "top_k": k}
    if filters:
        payload.update(filters)
    if max_distance is not None:
        payload["max_distance"] = max_distance
    data = _request("POST", "/api/v1/rag/chunks/search", payload)
    return [_document(chunk) for chunk in data.get("chunks", [])]


def list_by_metadata(filters: dict[str, str], k: int) -> list[Document]:
    payload: dict[str, Any] = {"top_k": k, **filters}
    data = _request("POST", "/api/v1/rag/chunks/list", payload)
    return [_document(chunk) for chunk in data.get("chunks", [])]


def count_chunks() -> int:
    data = _request("GET", "/api/v1/rag/chunks/count")
    return int(data.get("count", 0))


def drop_chunks() -> int:
    data = _request("DELETE", "/api/v1/rag/chunks")
    return int(data.get("deleted", 0))


def _document(chunk: dict[str, Any]) -> Document:
    metadata: dict[str, Any] = {
        "source": chunk.get("source", ""),
        "page": chunk.get("page", 0),
        "chunk": chunk.get("chunk", 0),
        "company_id": chunk.get("company_id", ""),
        "sector": chunk.get("sector", ""),
    }
    if chunk.get("published_at"):
        metadata["published_at"] = chunk["published_at"]
    return Document(page_content=chunk.get("content", ""), metadata=metadata)

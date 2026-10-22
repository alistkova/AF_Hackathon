import httpx
import pytest
from langchain_core.documents import Document

from app.rag_system.embeddings import (
    create_vector_store,
    load_vector_store,
    save_vector_store,
    search,
)
from app.rag_system.retrieve import format_context, source_labels


def test_vector_store_roundtrip(tmp_path) -> None:
    docs = [
        Document(
            page_content="Магнит расширяет логистику и открывает новые склады",
            metadata={"source": "news-1", "page": 1, "chunk": 0},
        ),
        Document(
            page_content="Банк России сохранил ключевую ставку на прежнем уровне",
            metadata={"source": "news-2", "page": 1, "chunk": 0},
        ),
    ]

    store = create_vector_store(docs, tmp_path)
    save_vector_store(store, tmp_path)
    loaded = load_vector_store(tmp_path)

    assert loaded is not None
    results = search(loaded, "Магнит логистика", k=1, score_threshold=0.0)
    assert results
    assert results[0].metadata["source"] == "news-1"


def test_update_index_refits_vocabulary(tmp_path, monkeypatch) -> None:
    from app.rag_system import pipeline

    first_docs = [
        Document(
            page_content="Магнит расширяет логистику",
            metadata={"source": "news-1", "page": 1, "chunk": 0},
        )
    ]
    store = create_vector_store(first_docs, tmp_path)
    save_vector_store(store, tmp_path)

    new_docs = [
        Document(
            page_content="Выручка компании выросла",
            metadata={"source": "news-2", "page": 1, "chunk": 0},
        )
    ]
    monkeypatch.setattr(
        pipeline,
        "ingest_pdfs_from_dir",
        lambda *_args: {
            "chunks": new_docs,
            "stats": {"new_files": 1, "total_chunks": 1, "skipped": 0},
        },
    )

    result = pipeline.update_index(data_dir=tmp_path, index_dir=tmp_path)

    assert result["status"] == "updated"
    loaded = load_vector_store(tmp_path)
    assert loaded is not None
    scored = loaded.similarity_search_with_score("выручка", k=2)
    assert scored[0][0].metadata["source"] == "news-2"
    assert scored[0][1] > 0


def test_single_document_index(tmp_path) -> None:
    docs = [
        Document(
            page_content="Единственный документ для проверки TF-IDF",
            metadata={"source": "news-1", "page": 1, "chunk": 0},
        )
    ]

    store = create_vector_store(docs, tmp_path)
    save_vector_store(store, tmp_path)
    loaded = load_vector_store(tmp_path)

    assert loaded is not None
    results = search(loaded, "документ", k=1, score_threshold=0.0)
    assert results


@pytest.mark.asyncio
async def test_rag_query_route(monkeypatch) -> None:
    from app.main import app
    from app.rag_system import pipeline

    def fake_query(question: str, **_kwargs):
        return {"question": question, "answer": "ok", "sources": ["news-1 (page 1)"]}

    monkeypatch.setattr(pipeline, "query_index", fake_query)

    transport = httpx.ASGITransport(app=app)
    async with httpx.AsyncClient(transport=transport, base_url="http://test") as client:
        response = await client.post("/v1/rag/query", json={"question": "?"})

    assert response.status_code == 200
    assert response.json() == {"answer": "ok", "sources": ["news-1 (page 1)"]}


def test_format_context_and_source_labels() -> None:
    docs = [
        Document(
            page_content="Текст новости",
            metadata={"source": "news-1", "page": 2, "chunk": 1},
        )
    ]

    context = format_context(docs)

    assert "[Source 1: news-1, page 2, chunk 1]" in context
    assert "Текст новости" in context
    assert source_labels(docs) == ["news-1 (page 2)"]

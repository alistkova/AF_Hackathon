import json

import httpx
import pytest
from langchain_core.documents import Document

import app.rag.routes as rag_routes
from app.main import app
from app.rag_system import store
from app.rag_system.ingest import _extract_date, _extract_entities, chunk_documents
from app.rag_system.retrieve import build_filter, format_context, source_labels
from app.rag_system.store import ContentStoreError


def mock_client(monkeypatch, handler) -> None:
    transport = httpx.MockTransport(handler)
    monkeypatch.setattr(
        store, "_client", lambda: httpx.Client(transport=transport, base_url="http://content")
    )


def test_build_filter_company() -> None:
    assert build_filter(company_id="magnit") == {"company_id": "magnit"}


def test_build_filter_sector() -> None:
    assert build_filter(sector="retail") == {"sector": "retail"}


def test_build_filter_date_after() -> None:
    assert build_filter(published_after="2026-01-01") == {"published_after": "2026-01-01"}


def test_build_filter_date_before() -> None:
    assert build_filter(published_before="2026-12-31") == {"published_before": "2026-12-31"}


def test_build_filter_date_range() -> None:
    expected = {"published_after": "2026-01-01", "published_before": "2026-12-31"}
    assert build_filter(published_after="2026-01-01", published_before="2026-12-31") == expected


def test_build_filter_all_fields() -> None:
    expected = {
        "company_id": "magnit",
        "sector": "retail",
        "published_after": "2026-01-01",
        "published_before": "2026-12-31",
    }
    actual = build_filter(
        company_id="magnit",
        sector="retail",
        published_after="2026-01-01",
        published_before="2026-12-31",
    )
    assert actual == expected


def test_build_filter_empty() -> None:
    assert build_filter() == {}


def test_store_search_posts_embedding_and_filters(monkeypatch) -> None:
    captured = {}

    def handler(request: httpx.Request) -> httpx.Response:
        captured["path"] = request.url.path
        captured["body"] = json.loads(request.content)
        return httpx.Response(
            200,
            json={
                "chunks": [
                    {
                        "content": "Магнит расширяет логистику",
                        "source": "news-1.pdf",
                        "page": 2,
                        "chunk": 1,
                        "company_id": "magnit",
                        "sector": "retail",
                        "published_at": "2026-03-15",
                        "distance": 0.21,
                    }
                ]
            },
        )

    mock_client(monkeypatch, handler)

    docs = store.search(
        [0.1, 0.2],
        k=3,
        max_distance=0.5,
        filters={"company_id": "magnit", "published_after": "2026-01-01"},
    )

    assert captured["path"] == "/api/v1/rag/chunks/search"
    assert captured["body"] == {
        "embedding": [0.1, 0.2],
        "top_k": 3,
        "company_id": "magnit",
        "published_after": "2026-01-01",
        "max_distance": 0.5,
    }
    assert len(docs) == 1
    assert docs[0].page_content == "Магнит расширяет логистику"
    assert docs[0].metadata == {
        "source": "news-1.pdf",
        "page": 2,
        "chunk": 1,
        "company_id": "magnit",
        "sector": "retail",
        "published_at": "2026-03-15",
    }


def test_store_search_without_optional_fields(monkeypatch) -> None:
    captured = {}

    def handler(request: httpx.Request) -> httpx.Response:
        captured["body"] = json.loads(request.content)
        return httpx.Response(200, json={"chunks": []})

    mock_client(monkeypatch, handler)

    assert store.search([0.1], k=5) == []
    assert captured["body"] == {"embedding": [0.1], "top_k": 5}


def test_store_add_chunks_maps_documents(monkeypatch) -> None:
    captured = {}

    def handler(request: httpx.Request) -> httpx.Response:
        captured["path"] = request.url.path
        captured["body"] = json.loads(request.content)
        return httpx.Response(200, json={"upserted": 2})

    mock_client(monkeypatch, handler)

    docs = [
        Document(
            page_content="Первый чанк",
            metadata={
                "source": "news-1.pdf",
                "page": 1,
                "chunk": 0,
                "company_id": "magnit",
                "sector": "retail",
                "published_at": "2026-03-15",
            },
        ),
        Document(page_content="Второй чанк", metadata={"source": "news-2.pdf", "page": 3}),
    ]

    upserted = store.add_chunks(docs, [[0.1, 0.2], [0.3, 0.4]])

    assert upserted == 2
    assert captured["path"] == "/api/v1/rag/chunks"
    chunks = captured["body"]["chunks"]
    assert chunks[0] == {
        "content": "Первый чанк",
        "source": "news-1.pdf",
        "page": 1,
        "chunk": 0,
        "company_id": "magnit",
        "sector": "retail",
        "published_at": "2026-03-15",
        "embedding": [0.1, 0.2],
    }
    assert chunks[1]["published_at"] is None
    assert chunks[1]["company_id"] == ""
    assert chunks[1]["chunk"] == 0


def test_store_add_chunks_rejects_mismatched_vectors() -> None:
    with pytest.raises(ValueError):
        store.add_chunks([Document(page_content="text")], [])


def test_store_list_by_metadata(monkeypatch) -> None:
    captured = {}

    def handler(request: httpx.Request) -> httpx.Response:
        captured["path"] = request.url.path
        captured["body"] = json.loads(request.content)
        return httpx.Response(200, json={"chunks": [{"content": "ctx", "source": "news-1.pdf"}]})

    mock_client(monkeypatch, handler)

    docs = store.list_by_metadata({"sector": "retail"}, 7)

    assert captured["path"] == "/api/v1/rag/chunks/list"
    assert captured["body"] == {"top_k": 7, "sector": "retail"}
    assert docs[0].page_content == "ctx"


def test_store_count_and_drop(monkeypatch) -> None:
    paths = []

    def handler(request: httpx.Request) -> httpx.Response:
        paths.append((request.method, request.url.path))
        if request.method == "DELETE":
            return httpx.Response(200, json={"deleted": 4})
        return httpx.Response(200, json={"count": 4})

    mock_client(monkeypatch, handler)

    assert store.count_chunks() == 4
    assert store.drop_chunks() == 4
    assert paths == [
        ("GET", "/api/v1/rag/chunks/count"),
        ("DELETE", "/api/v1/rag/chunks"),
    ]


def test_store_raises_on_error_status(monkeypatch) -> None:
    def handler(_request: httpx.Request) -> httpx.Response:
        return httpx.Response(500, json={"code": "content_unavailable", "message": "boom"})

    mock_client(monkeypatch, handler)

    with pytest.raises(ContentStoreError) as error:
        store.count_chunks()

    assert error.value.status_code == 500


def test_store_raises_on_connection_failure(monkeypatch) -> None:
    def handler(_request: httpx.Request) -> httpx.Response:
        raise httpx.ConnectError("no route")

    mock_client(monkeypatch, handler)

    with pytest.raises(ContentStoreError) as error:
        store.count_chunks()

    assert error.value.status_code is None


def test_extract_entities_single_company() -> None:
    entities = _extract_entities("Магнит развивает розничную сеть супермаркетов")
    assert entities == {"company_id": "magnit", "sector": "retail"}


def test_extract_entities_picks_first_company() -> None:
    entities = _extract_entities("Магнит и X5 Group развивают сеть супермаркетов")
    assert entities["company_id"] == "magnit"
    assert entities["sector"] == "retail"


def test_extract_entities_unknown() -> None:
    entities = _extract_entities("Какая-то абстрактная новость без компаний")
    assert entities == {"company_id": "unknown", "sector": "general"}


def test_extract_date_iso() -> None:
    assert _extract_date("Отчёт за 2026-03-15 опубликован") == "2026-03-15"


def test_extract_date_ddmmyyyy() -> None:
    assert _extract_date("Событие произошло 15.03.2026 года") == "2026-03-15"


def test_extract_date_none() -> None:
    assert _extract_date("Новость без даты") is None


def test_chunk_documents_enriches_metadata() -> None:
    docs = [
        Document(
            page_content="Магнит развивает розничную сеть супермаркетов 2026-03-15",
            metadata={"source": "news-1", "page": 1},
        )
    ]

    chunks = chunk_documents(docs)

    assert len(chunks) == 1
    metadata = chunks[0].metadata
    assert metadata["company_id"] == "magnit"
    assert metadata["sector"] == "retail"
    assert metadata["published_at"] == "2026-03-15"
    assert metadata["chunk"] == 0
    assert metadata["source"] == "news-1"
    assert metadata["page"] == 1


def test_chunk_documents_multiple_chunks() -> None:
    docs = [
        Document(
            page_content="Магнит развивает розничную сеть супермаркетов " * 200,
            metadata={"source": "news-1", "page": 1},
        )
    ]

    chunks = chunk_documents(docs)

    assert len(chunks) > 1
    for index, chunk in enumerate(chunks):
        assert chunk.metadata["chunk"] == index
        assert chunk.metadata["company_id"] == "magnit"
        assert chunk.metadata["sector"] == "retail"


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


def test_format_context_empty() -> None:
    assert format_context([]) == "No relevant information found."


@pytest.mark.asyncio
async def test_query_route(monkeypatch) -> None:
    def fake_query(question: str, **_kwargs):
        return {"question": question, "answer": "ok", "sources": ["news-1 (page 1)"]}

    monkeypatch.setattr(rag_routes, "query_index", fake_query)

    transport = httpx.ASGITransport(app=app)
    async with httpx.AsyncClient(transport=transport, base_url="http://test") as client:
        response = await client.post("/v1/rag/query", json={"question": "?"})

    assert response.status_code == 200
    assert response.json() == {"answer": "ok", "sources": ["news-1 (page 1)"]}


@pytest.mark.asyncio
async def test_search_route(monkeypatch) -> None:
    def fake_search(query: str, **_kwargs):
        return {
            "query": query,
            "chunks": [{"content": "Магнит развивает сеть супермаркетов"}],
            "sources": ["news-1 (page 1)"],
        }

    monkeypatch.setattr(rag_routes, "search_chunks", fake_search)

    transport = httpx.ASGITransport(app=app)
    async with httpx.AsyncClient(transport=transport, base_url="http://test") as client:
        response = await client.post(
            "/v1/rag/search", json={"query": "логистика", "company_id": "magnit"}
        )

    assert response.status_code == 200
    assert response.json()["query"] == "логистика"
    assert response.json()["chunks"][0]["content"] == "Магнит развивает сеть супермаркетов"


@pytest.mark.asyncio
async def test_list_chunks_route(monkeypatch) -> None:
    def fake_list(**_kwargs):
        return {
            "chunks": [{"content": "Магнит открывает склад", "published_at": "2026-01-15"}],
            "sources": ["news-1 (page 1)"],
        }

    monkeypatch.setattr(rag_routes, "list_chunks", fake_list)

    transport = httpx.ASGITransport(app=app)
    async with httpx.AsyncClient(transport=transport, base_url="http://test") as client:
        response = await client.post("/v1/rag/list-chunks", json={"company_id": "magnit"})

    assert response.status_code == 200
    assert response.json()["chunks"][0]["published_at"] == "2026-01-15"


@pytest.mark.asyncio
async def test_search_route_reports_content_unavailable(monkeypatch) -> None:
    def fake_search(*_args, **_kwargs):
        raise ContentStoreError("content service is unavailable")

    monkeypatch.setattr(rag_routes, "search_chunks", fake_search)

    transport = httpx.ASGITransport(app=app)
    async with httpx.AsyncClient(transport=transport, base_url="http://test") as client:
        response = await client.post("/v1/rag/search", json={"query": "логистика"})

    assert response.status_code == 503


@pytest.mark.asyncio
async def test_search_route_maps_content_rejection_to_400(monkeypatch) -> None:
    def fake_search(*_args, **_kwargs):
        raise ContentStoreError("bad date", status_code=400)

    monkeypatch.setattr(rag_routes, "search_chunks", fake_search)

    transport = httpx.ASGITransport(app=app)
    async with httpx.AsyncClient(transport=transport, base_url="http://test") as client:
        response = await client.post(
            "/v1/rag/search", json={"query": "логистика", "published_after": "31.12.2026"}
        )

    assert response.status_code == 400

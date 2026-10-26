import pytest

from app.contracts.models import AnalysisRequest
from app.pipeline import enrichment


def payload(evidence_count: int = 1) -> AnalysisRequest:
    ev = [
        {"source_id": f"n{i}", "title": f"t{i}", "url": f"https://x/{i}"}
        for i in range(evidence_count)
    ]
    return AnalysisRequest.model_validate(
        {
            "request_id": "t",
            "company": {"id": "magnit", "name": "Магнит", "industry": "Ритейл"},
            "event": {"id": "e1", "title": "ЦБ сохранил ставку"},
            "evidence": ev,
        }
    )


@pytest.fixture(autouse=True)
def no_news(monkeypatch):
    """По умолчанию новости не тянем — тесты не должны ходить в сеть."""

    async def fake_news(_payload):
        return []

    monkeypatch.setattr(enrichment, "_news_fallback", fake_news)


@pytest.mark.asyncio
async def test_skip_when_evidence_is_enough(monkeypatch):
    called = {"search": 0}

    def fake_search(**_):
        called["search"] += 1
        return {"chunks": []}

    monkeypatch.setattr(enrichment, "search_chunks", fake_search)
    result = await enrichment.enrich_evidence(payload(evidence_count=5))
    assert len(result.evidence) == 5
    assert called["search"] == 0


@pytest.mark.asyncio
async def test_adds_chunks_from_company_search(monkeypatch):
    def fake_search(**kwargs):
        if kwargs.get("company_id"):
            return {
                "chunks": [
                    {"content": "ctx1", "source": "news.pdf", "page": 2, "chunk": 1},
                ]
            }
        return {"chunks": []}

    monkeypatch.setattr(enrichment, "search_chunks", fake_search)
    monkeypatch.setattr(enrichment, "list_chunks", lambda **_: {"chunks": []})

    result = await enrichment.enrich_evidence(payload(evidence_count=1))
    ids = {e.source_id for e in result.evidence}
    assert "rag:news.pdf:2:1" in ids


@pytest.mark.asyncio
async def test_fallback_to_global_search(monkeypatch):
    calls = []

    def fake_search(**kwargs):
        calls.append(kwargs)
        if kwargs.get("company_id"):
            return {"chunks": []}
        return {"chunks": [{"content": "global ctx", "source": "g.pdf", "page": 1, "chunk": 0}]}

    monkeypatch.setattr(enrichment, "search_chunks", fake_search)
    monkeypatch.setattr(enrichment, "list_chunks", lambda **_: {"chunks": []})

    result = await enrichment.enrich_evidence(payload(evidence_count=1))
    assert any("rag:g.pdf" in e.source_id for e in result.evidence)
    assert len(calls) >= 2
    assert calls[0].get("company_id") == "magnit"
    assert calls[1].get("company_id") is None


@pytest.mark.asyncio
async def test_caps_at_max(monkeypatch):
    def fake_search(**_):
        return {
            "chunks": [
                {"content": f"c{i}", "source": "s.pdf", "page": 1, "chunk": i} for i in range(20)
            ]
        }

    monkeypatch.setattr(enrichment, "search_chunks", fake_search)
    monkeypatch.setattr(enrichment, "list_chunks", lambda **_: {"chunks": []})

    result = await enrichment.enrich_evidence(payload(evidence_count=1))
    assert len(result.evidence) == enrichment.MAX_EVIDENCE


@pytest.mark.asyncio
async def test_search_failure_is_graceful(monkeypatch):
    def boom(**_):
        raise RuntimeError("content down")

    monkeypatch.setattr(enrichment, "search_chunks", boom)
    monkeypatch.setattr(enrichment, "list_chunks", boom)

    result = await enrichment.enrich_evidence(payload(evidence_count=1))
    assert len(result.evidence) == 1


@pytest.mark.asyncio
async def test_news_fallback_when_all_empty(monkeypatch):
    monkeypatch.setattr(enrichment, "search_chunks", lambda **_: {"chunks": []})
    monkeypatch.setattr(enrichment, "list_chunks", lambda **_: {"chunks": []})

    from app.contracts.models import Evidence

    async def fake_news(_payload):
        return [
            Evidence(source_id="news:a1", title="n1", url="https://n/1", snippet="x"),
            Evidence(source_id="news:a2", title="n2", url="https://n/2", snippet="y"),
        ]

    monkeypatch.setattr(enrichment, "_news_fallback", fake_news)

    result = await enrichment.enrich_evidence(payload(evidence_count=1))
    assert len(result.evidence) == 3
    assert any(e.source_id == "news:a1" for e in result.evidence)

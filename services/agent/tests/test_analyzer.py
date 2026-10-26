import json

import pytest

from app.contracts.models import AnalysisRequest, Insight, ResultEvent
from app.pipeline import cache
from app.pipeline.analyzer import StubAnalyzer


@pytest.mark.asyncio
async def test_stub_analyzer_returns_cited_insight() -> None:
    payload = AnalysisRequest.model_validate(
        {
            "request_id": "test-job",
            "company": {"id": "magnit", "name": "Магнит"},
            "event": {"id": "news-001", "title": "Магнит расширяет логистику"},
            "evidence": [
                {
                    "source_id": "news-001",
                    "title": "Магнит расширяет логистику",
                    "url": "https://example.com/news-001",
                }
            ],
        }
    )

    events = [event async for event in StubAnalyzer(delay=0).analyze(payload)]

    assert events[0]["event"] == "step"
    assert events[-1]["event"] == "result"
    result = ResultEvent.model_validate_json(events[-1]["data"])
    assert result.insight.sources[0].source_id == "news-001"
    assert isinstance(result.insight, Insight)
    assert json.loads(events[-1]["data"])["insight"]["summary"].startswith("Магнит")


@pytest.mark.asyncio
async def test_stub_analyzer_satisfies_validators() -> None:
    from app.contracts.models import AnalysisRequest, ResultEvent
    from app.pipeline.analyzer import StubAnalyzer
    from app.pipeline.metrics import evaluate_event
    from app.pipeline.validators import validate_insight

    payload = AnalysisRequest.model_validate(
        {
            "request_id": "test-job",
            "company": {"id": "magnit", "name": "Магнит"},
            "event": {"id": "news-001", "title": "Магнит расширяет логистику"},
            "evidence": [
                {
                    "source_id": "news-001",
                    "title": "Магнит расширяет логистику",
                    "url": "https://example.com/news-001",
                },
                {
                    "source_id": "rag:ALFA-Macro_Insights.pdf:1:0",
                    "title": "Alfa Macro",
                    "url": "rag://macro",
                },
            ],
        }
    )

    events = [event async for event in StubAnalyzer(delay=0).analyze(payload)]
    insight = ResultEvent.model_validate_json(events[-1]["data"]).insight

    assert validate_insight(insight, payload) == []
    assert insight.bank_view.used is True
    assert insight.bank_view.source_ids == ["rag:ALFA-Macro_Insights.pdf:1:0"]

    metrics = evaluate_event(insight, payload)
    assert metrics.source_coverage == 1.0
    assert metrics.separation_ok is True
    assert metrics.issues == []


@pytest.mark.asyncio
async def test_stub_analyzer_skips_bank_view_without_alfa() -> None:
    from app.contracts.models import AnalysisRequest, ResultEvent
    from app.pipeline.analyzer import StubAnalyzer
    from app.pipeline.validators import validate_insight

    payload = AnalysisRequest.model_validate(
        {
            "request_id": "test-job",
            "company": {"id": "magnit", "name": "Магнит"},
            "event": {"id": "news-001", "title": "Магнит расширяет логистику"},
            "evidence": [
                {
                    "source_id": "news-001",
                    "title": "Магнит расширяет логистику",
                    "url": "https://example.com/news-001",
                }
            ],
        }
    )

    events = [event async for event in StubAnalyzer(delay=0).analyze(payload)]
    insight = ResultEvent.model_validate_json(events[-1]["data"]).insight

    assert validate_insight(insight, payload) == []
    assert insight.bank_view.used is False
    assert insight.bank_position == ""


@pytest.mark.asyncio
async def test_cache_hit_skips_llm(monkeypatch):
    from app.contracts.models import AnalysisRequest
    from app.pipeline.analyzer import LLMAnalyzer

    cache.clear()

    class FakeClient:
        def __init__(self):
            self.calls = 0

        async def complete_json(self, system, user):
            self.calls += 1
            return (
                {
                    "summary": "s",
                    "why_it_matters": "w",
                    "risks": ["r"],
                    "opportunities": ["o"],
                    "bank_position": "",
                    "ai_conclusion": "a",
                    "sources": [{"source_id": "n1", "title": "t", "url": "https://x"}],
                    "actions": ["a"],
                    "confidence": 0.5,
                    "facts": [{"text": "f", "source_ids": ["n1"], "confidence": "high"}],
                },
                {"prompt_tokens": 10, "completion_tokens": 5},
            )

    client = FakeClient()
    analyzer = LLMAnalyzer(client=client, model="fake")

    p = AnalysisRequest.model_validate(
        {
            "request_id": "t",
            "company": {"id": "magnit", "name": "Магнит"},
            "event": {"id": "e1", "title": "t"},
            "evidence": [{"source_id": "n1", "title": "t", "url": "https://x"}],
        }
    )

    insight1, _ = await analyzer._generate(p)
    assert client.calls == 1
    assert insight1.meta.get("cache") == "miss"

    insight2, _ = await analyzer._generate(p)
    assert client.calls == 1
    assert insight2.meta.get("cache") == "hit"

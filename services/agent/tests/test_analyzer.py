import json

import pytest

from app.contracts.models import AnalysisRequest, Insight, ResultEvent
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

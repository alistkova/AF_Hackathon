import pytest

from app.contracts.models import AnalysisRequest
from app.pipeline import cache
from app.pipeline.analyzer import LLMAnalyzer


def make_payload(alfa_in_evidence: bool) -> AnalysisRequest:
    ev = [{"source_id": "n1", "title": "news", "url": "https://n/1"}]
    if alfa_in_evidence:
        ev.append(
            {
                "source_id": "rag:ALFA-Macro_Insights-Policy_Rate_11Sep.pdf:1:0",
                "title": "alfa",
                "url": "rag://a",
            }
        )
    return AnalysisRequest.model_validate(
        {
            "request_id": "t",
            "company": {"id": "magnit", "name": "Магнит", "industry": "Ритейл", "region": "Москва"},
            "event": {"id": "e1", "title": "ЦБ сохранил ставку"},
            "evidence": ev,
        }
    )


@pytest.mark.asyncio
async def test_bank_view_used_when_alfa_present():
    cache.clear()

    class FakeClient:
        async def complete_json(self, system, user):
            return (
                {
                    "summary": "s",
                    "why_it_matters": "w",
                    "risks": ["r"],
                    "opportunities": ["o"],
                    "bank_position": "позиция банка",
                    "ai_conclusion": "вывод",
                    "sources": [
                        {"source_id": "n1", "title": "n", "url": "https://n/1"},
                        {
                            "source_id": "rag:ALFA-Macro_Insights-Policy_Rate_11Sep.pdf:1:0",
                            "title": "a",
                            "url": "rag://a",
                        },
                    ],
                    "actions": ["a"],
                    "confidence": 0.7,
                    "facts": [{"text": "f", "source_ids": ["n1"], "confidence": "high"}],
                    "bank_view": {
                        "used": True,
                        "text": "позиция банка",
                        "source_ids": ["rag:ALFA-Macro_Insights-Policy_Rate_11Sep.pdf:1:0"],
                    },
                    "ai_view": {"text": "вывод", "source_ids": ["n1"]},
                    "meta": {"signals_used": ["industry:Ритейл"]},
                },
                {"prompt_tokens": 10, "completion_tokens": 5},
            )

    analyzer = LLMAnalyzer(client=FakeClient(), model="fake")
    insight, issues = await analyzer._generate(make_payload(alfa_in_evidence=True))
    assert issues == []
    assert insight.bank_view.used is True
    assert insight.meta.get("alfa_analytics_sources")
    assert insight.meta.get("signals_used") == ["industry:Ритейл"]


@pytest.mark.asyncio
async def test_bank_view_with_non_alfa_source_fails_validation():
    cache.clear()

    class FakeClient:
        async def complete_json(self, system, user):
            return (
                {
                    "summary": "s",
                    "why_it_matters": "w",
                    "risks": ["r"],
                    "opportunities": ["o"],
                    "bank_position": "позиция",
                    "ai_conclusion": "вывод",
                    "sources": [{"source_id": "n1", "title": "n", "url": "https://n/1"}],
                    "actions": ["a"],
                    "confidence": 0.5,
                    "facts": [{"text": "f", "source_ids": ["n1"], "confidence": "high"}],
                    "bank_view": {"used": True, "text": "позиция", "source_ids": ["n1"]},
                    "ai_view": {"text": "вывод", "source_ids": ["n1"]},
                    "meta": {},
                },
                None,
            )

    analyzer = LLMAnalyzer(client=FakeClient(), model="fake")
    _, issues = await analyzer._generate(make_payload(alfa_in_evidence=False))
    codes = [i.code for i in issues]
    assert "bank_view_no_alfa_source" in codes

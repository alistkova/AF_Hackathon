from app.contracts.models import AnalysisRequest, CitedClaim, Insight, Source
from app.pipeline.validators import validate_insight


def payload():
    return AnalysisRequest.model_validate(
        {
            "request_id": "t",
            "company": {"id": "magnit", "name": "Магнит"},
            "event": {"id": "n1", "title": "Событие"},
            "evidence": [{"source_id": "n1", "title": "t", "url": "https://x"}],
        }
    )


def insight(**overrides):
    base = dict(
        summary="S",
        why_it_matters="W",
        risks=["r"],
        opportunities=["o"],
        bank_position="",
        ai_conclusion="ai",
        sources=[Source(source_id="n1", title="t", url="https://x")],
        actions=["a"],
        confidence=0.7,
        facts=[CitedClaim(text="f", source_ids=["n1"])],
    )
    base.update(overrides)
    return Insight(**base)


def codes(i: Insight) -> list[str]:
    return [x.code for x in validate_insight(i, payload())]


def test_valid_insight_passes():
    assert validate_insight(insight(), payload()) == []


def test_unknown_source_is_caught():
    assert "unknown_source" in codes(
        insight(facts=[CitedClaim(text="f", source_ids=["ghost"])])
    )


def test_missing_disclaimer():
    assert "missing_disclaimer" in codes(insight(disclaimer=""))


def test_bank_view_without_source():
    assert "bank_view_without_source" in codes(
        insight(bank_view={"used": True, "text": "x", "source_ids": []})
    )


def test_bank_view_leak():
    assert "bank_view_leak" in codes(insight(bank_position="позиция банка"))


def test_empty_claims():
    assert "empty_claims" in codes(insight(facts=[], hypotheses=[]))
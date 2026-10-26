from app.contracts.models import (
    AIView, AnalysisRequest, BankView, CitedClaim, Insight, Source,
)
from app.pipeline.metrics import (
    aggregate, check_thresholds, evaluate_event, has_alfa_in_evidence,
    separation_ok, source_coverage,
)


def payload(alfa: bool = True) -> AnalysisRequest:
    ev = [{"source_id": "n1", "title": "t", "url": "https://x"}]
    if alfa:
        ev.append({"source_id": "rag:ALFA-Macro_Insights-Policy_Rate_11Sep.pdf:1:0",
                   "title": "a", "url": "rag://a"})
    return AnalysisRequest.model_validate({
        "request_id": "t",
        "company": {"id": "magnit", "name": "Магнит"},
        "event": {"id": "e", "title": "t"},
        "evidence": ev,
    })


def insight(**over) -> Insight:
    base = dict(
        summary="s", why_it_matters="w",
        risks=["r"], opportunities=["o"],
        bank_position="", ai_conclusion="a",
        sources=[Source(source_id="n1", title="t", url="https://x")],
        actions=["a"], confidence=0.5,
        facts=[CitedClaim(text="f", source_ids=["n1"])],
        ai_view=AIView(text="ai", source_ids=["n1"]),
    )
    base.update(over)
    return Insight(**base)


def test_source_coverage_full():
    assert source_coverage(insight()) == 1.0


def test_source_coverage_partial():
    i = insight(facts=[
        CitedClaim(text="f", source_ids=["n1"]),
        CitedClaim(text="f2", source_ids=[]),
    ])
    assert source_coverage(i) == 0.5


def test_separation_ok_when_unused_and_empty():
    assert separation_ok(insight()) is True


def test_separation_bad_when_unused_but_text():
    assert separation_ok(insight(bank_position="позиция")) is False


def test_separation_bad_when_used_non_alfa():
    i = insight(bank_view=BankView(used=True, text="x", source_ids=["n1"]))
    assert separation_ok(i) is False


def test_separation_ok_when_used_alfa():
    i = insight(bank_view=BankView(
        used=True, text="x",
        source_ids=["rag:ALFA-Macro_Insights-Policy_Rate_11Sep.pdf:1:0"],
    ))
    assert separation_ok(i) is True


def test_has_alfa_detection():
    assert has_alfa_in_evidence(payload(alfa=True)) is True
    assert has_alfa_in_evidence(payload(alfa=False)) is False


def test_evaluate_flags_alfa_not_used():
    m = evaluate_event(insight(), payload(alfa=True))
    assert "alfa_present_but_not_used" in m.issues


def test_evaluate_flags_used_without_alfa():
    p = payload(alfa=False)
    i = insight(bank_view=BankView(used=True, text="x", source_ids=["n1"]))
    m = evaluate_event(i, p)
    assert any("separation" in x or "bank_view" in x for x in m.issues)


def test_evaluate_flags_degraded():
    m = evaluate_event(insight(meta={"degraded": True}), payload(alfa=False))
    assert "degraded" in m.issues


def test_aggregate_and_thresholds():
    rows = [
        evaluate_event(insight(), payload(alfa=False)),
        evaluate_event(
            insight(bank_view=BankView(
                used=True, text="x",
                source_ids=["rag:ALFA-Macro_Insights-Policy_Rate_11Sep.pdf:1:0"],
                ), bank_position="x"),
            payload(alfa=True),
        ),
    ]
    agg = aggregate(rows)
    assert agg["total"] == 2
    assert check_thresholds(agg) == []

import json

from app.contracts.models import (
    AIView,
    AnalysisRequest,
    BankView,
    CitedClaim,
    Insight,
    Source,
)
from app.pipeline.metrics import (
    aggregate,
    check_thresholds,
    evaluate_event,
    has_alfa_in_evidence,
    separation_ok,
    source_coverage,
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


def test_recorder_writes_jsonl_line(tmp_path):
    from app.metrics.recorder import MetricsRecorder, _FILENAME

    sink = MetricsRecorder(directory=tmp_path, enabled=True)
    sink.record(
        source="agent.analyze", model="m", e2el_s=2.0,
        input_tokens=100, output_tokens=50, total_tokens=150,
        output_tput=25.0, total_tput=75.0, ok=True, error=None,
    )
    target = tmp_path / _FILENAME
    lines = target.read_text(encoding="utf-8").strip().splitlines()
    assert len(lines) == 1
    entry = json.loads(lines[0])
    assert entry["source"] == "agent.analyze"
    assert entry["output_tput"] == 25.0
    assert entry["total_tput"] == 75.0
    assert entry["ok"] is True
    assert "ts" in entry


def test_recorder_disabled_writes_nothing(tmp_path):
    from app.metrics.recorder import MetricsRecorder, _FILENAME

    sink = MetricsRecorder(directory=tmp_path, enabled=False)
    sink.record(source="rag.query", model="m", e2el_s=1.0)
    assert not (tmp_path / _FILENAME).exists()


def test_recorder_does_not_fail_on_bad_dir(tmp_path):
    from app.metrics.recorder import MetricsRecorder, _FILENAME

    bad = tmp_path / "no" / "such" / "nest"
    sink = MetricsRecorder(directory=bad, enabled=True)
    sink.record(source="agent.analyze", model="m", e2el_s=1.0)
    assert (bad / _FILENAME).exists()


def test_aggregate_summarizes(tmp_path):
    from app.metrics.aggregate import summarize
    from app.metrics.recorder import MetricsRecorder, _FILENAME

    sink = MetricsRecorder(directory=tmp_path, enabled=True)
    for i in range(1, 4):
        sink.record(
            source="rag.query", model="m", e2el_s=float(i),
            input_tokens=10, output_tokens=5, total_tokens=15,
            output_tput=1.0, total_tput=3.0, ok=True,
        )
    result = summarize(tmp_path / _FILENAME, group="source")
    assert result["total_entries"] == 3
    rows = [g for g in result["groups"] if g["group"] == ("rag.query",)]
    assert rows[0]["count"] == 3
    assert rows[0]["e2el_median_s"] == 2.0
    assert rows[0]["output_tokens_total"] == 15.0

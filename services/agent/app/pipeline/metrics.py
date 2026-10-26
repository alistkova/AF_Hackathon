"""Метрики качества инсайта. Чистые функции — без LLM и сети."""
from __future__ import annotations

from dataclasses import dataclass, field

from app.contracts.models import AnalysisRequest, Insight
from app.llm.sources import is_alfa_source


@dataclass
class EventMetrics:
    source_coverage: float
    separation_ok: bool
    bank_view_used: bool
    bank_view_expected: bool
    disclaimer_ok: bool
    degraded: bool
    issues: list[str] = field(default_factory=list)


def _claims(insight: Insight):
    return (
        insight.facts
        + insight.hypotheses
        + insight.structured_risks
        + insight.structured_opportunities
    )


def source_coverage(insight: Insight) -> float:
    claims = _claims(insight)
    if not claims:
        return 0.0
    cited = sum(1 for c in claims if c.source_ids)
    return cited / len(claims)


def separation_ok(insight: Insight) -> bool:
    """bank_view и legacy bank_position не противоречат друг другу."""
    if insight.bank_view.used:
        if not insight.bank_view.source_ids:
            return False
        if not all(is_alfa_source(sid) for sid in insight.bank_view.source_ids):
            return False
        return True
    # used=false => bank_view.text и bank_position должны быть пустыми
    if insight.bank_view.text.strip():
        return False
    if insight.bank_position.strip():
        return False
    return True


def has_alfa_in_evidence(payload: AnalysisRequest) -> bool:
    return any(is_alfa_source(e.source_id) for e in payload.evidence)


def evaluate_event(
    insight: Insight, payload: AnalysisRequest
) -> EventMetrics:
    issues: list[str] = []
    cov = source_coverage(insight)
    sep = separation_ok(insight)
    used = insight.bank_view.used
    expected = has_alfa_in_evidence(payload)
    disc = bool(insight.disclaimer.strip())
    degraded = bool(insight.meta.get("degraded"))

    if cov < 0.9:
        issues.append(f"low_source_coverage:{cov:.2f}")
    if not sep:
        issues.append("separation_violated")
    if expected and not used:
        issues.append("alfa_present_but_not_used")
    if not expected and used:
        issues.append("bank_view_used_without_alfa")
    if not disc:
        issues.append("missing_disclaimer")
    if degraded:
        issues.append("degraded")

    return EventMetrics(
        source_coverage=cov,
        separation_ok=sep,
        bank_view_used=used,
        bank_view_expected=expected,
        disclaimer_ok=disc,
        degraded=degraded,
        issues=issues,
    )


def aggregate(rows: list[EventMetrics]) -> dict:
    if not rows:
        return {}
    n = len(rows)
    expected_rows = [r for r in rows if r.bank_view_expected]
    used_when_expected = (
        sum(1 for r in expected_rows if r.bank_view_used) / len(expected_rows)
        if expected_rows else 1.0
    )
    return {
        "total": n,
        "source_coverage_avg": round(sum(r.source_coverage for r in rows) / n, 3),
        "separation_ok_rate": round(sum(r.separation_ok for r in rows) / n, 3),
        "bank_view_used_when_expected_rate": round(used_when_expected, 3),
        "disclaimer_ok_rate": round(sum(r.disclaimer_ok for r in rows) / n, 3),
        "degraded_rate": round(sum(r.degraded for r in rows) / n, 3),
        "events_with_issues": sum(1 for r in rows if r.issues),
    }


THRESHOLDS = {
    "source_coverage_avg": 0.9,
    "separation_ok_rate": 0.9,
    "bank_view_used_when_expected_rate": 0.7,
    "disclaimer_ok_rate": 1.0,
    "degraded_rate_max": 0.1,
}


def check_thresholds(agg: dict) -> list[str]:
    failures = []
    for key, threshold in THRESHOLDS.items():
        if key == "degraded_rate_max":
            if agg.get("degraded_rate", 0) > threshold:
                failures.append(f"degraded_rate {agg['degraded_rate']} > {threshold}")
            continue
        actual = agg.get(key, 0)
        if actual < threshold:
            failures.append(f"{key} {actual} < {threshold}")
    return failures

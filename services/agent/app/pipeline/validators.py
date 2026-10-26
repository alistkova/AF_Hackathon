"""Детерминированные проверки инсайта перед отправкой клиенту."""
from __future__ import annotations
from app.llm.prompts import is_alfa_source
from dataclasses import dataclass

from app.contracts.models import AnalysisRequest, Insight


@dataclass
class ValidationIssue:
    code: str
    message: str


def validate_insight(insight: Insight, payload: AnalysisRequest) -> list[ValidationIssue]:
    issues: list[ValidationIssue] = []
    known = {item.source_id for item in payload.evidence}

    def check_ids(ids: list[str], where: str) -> None:
        for sid in ids:
            if sid not in known:
                issues.append(
                    ValidationIssue(
                        code="unknown_source",
                        message=f"{where}: source_id '{sid}' отсутствует в evidence",
                    )
                )

    for i, c in enumerate(insight.facts):
        check_ids(c.source_ids, f"facts[{i}]")
    for i, c in enumerate(insight.hypotheses):
        check_ids(c.source_ids, f"hypotheses[{i}]")
    for i, c in enumerate(insight.structured_risks):
        check_ids(c.source_ids, f"structured_risks[{i}]")
    for i, c in enumerate(insight.structured_opportunities):
        check_ids(c.source_ids, f"structured_opportunities[{i}]")
    check_ids(insight.bank_view.source_ids, "bank_view")
    check_ids(insight.ai_view.source_ids, "ai_view")
    check_ids([s.source_id for s in insight.sources], "sources")

    claims = (
        insight.facts
        + insight.hypotheses
        + insight.structured_risks
        + insight.structured_opportunities
    )
    if payload.evidence and not any(c.source_ids for c in claims):
        issues.append(
            ValidationIssue(
                code="no_citations",
                message="Ни один вывод не привязан к источнику, хотя evidence не пуст",
            )
        )

    if not insight.disclaimer.strip():
        issues.append(ValidationIssue("missing_disclaimer", "disclaimer обязателен"))

    # 1. used=true, но source_ids пуст — это ошибка
    if insight.bank_view.used and not insight.bank_view.source_ids:
        issues.append(
            ValidationIssue(
                code="bank_view_without_source",
                message="bank_view.used=true, но source_ids пуст",
            )
        )

    # 2. used=true и source_ids непуст, но ни один не Alfa — ошибка
    if insight.bank_view.used and insight.bank_view.source_ids:
        if not any(is_alfa_source(sid) for sid in insight.bank_view.source_ids):
            issues.append(
                ValidationIssue(
                    code="bank_view_no_alfa_source",
                    message=(
                        "bank_view.used=true, но ни один source_id не относится "
                        "к аналитике Альфа-Банка"
                    ),
                )
            )

    # 3. used=false, но legacy bank_position заполнен — leak
    if not insight.bank_view.used and insight.bank_position.strip():
        issues.append(
            ValidationIssue(
                code="bank_view_leak",
                message=(
                    "bank_view.used=false, но legacy bank_position заполнен. "
                    "Либо перенеси текст в ai_view, либо поставь used=true с source_ids."
                ),
            )
        )

    if not insight.facts and not insight.hypotheses:
        issues.append(
            ValidationIssue(
                code="empty_claims",
                message="Нужно хотя бы одно утверждение в facts или hypotheses",
            )
        )

    return issues


def format_issues(issues: list[ValidationIssue]) -> str:
    return "\n".join(f"- [{i.code}] {i.message}" for i in issues)

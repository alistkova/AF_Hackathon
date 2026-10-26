import asyncio
import json
from collections.abc import AsyncIterator
from typing import Protocol

from app.config import Settings
from app.contracts.models import (
    AnalysisRequest,
    Insight,
    ResultEvent,
    Source,
    StepEvent,
)
from app.llm.openai_compat import OpenAICompatClient
from app.llm.prompts import SYSTEM_PROMPT, build_repair_prompt
from app.pipeline.validators import ValidationIssue, format_issues, validate_insight

AgentEvent = dict[str, str]


class Analyzer(Protocol):
    def analyze(self, payload: AnalysisRequest) -> AsyncIterator[AgentEvent]: ...


def step(name: str, status: str, message: str) -> AgentEvent:
    event = StepEvent(name=name, status=status, message=message)
    return {"event": "step", "data": event.model_dump_json()}


def result(insight: Insight) -> AgentEvent:
    event = ResultEvent(insight=insight, meta=insight.meta)
    return {"event": "result", "data": event.model_dump_json()}


class StubAnalyzer:
    def __init__(self, delay: float = 0.35) -> None:
        self.delay = delay

    async def analyze(self, payload: AnalysisRequest) -> AsyncIterator[AgentEvent]:
        stages = [
            ("retrieving", "Ищем связанные новости и документы"),
            ("ranking", "Оцениваем влияние на компанию"),
            ("analyzing", "Готовим вывод AI и позицию банка"),
        ]
        for name, message in stages:
            yield step(name, "running", message)
            await asyncio.sleep(self.delay)
            yield step(name, "done", message)

        sources = [
            Source(source_id=item.source_id, title=item.title, url=item.url)
            for item in payload.evidence
        ]
        insight = Insight(
            summary=f"{payload.company.name}: {payload.event.title}",
            why_it_matters=(
                "Событие влияет на операционную эффективность, стоимость капитала "
                "и переговорную позицию компании."
            ),
            risks=[
                "Рост затрат может ограничить свободный денежный поток.",
                "Сроки реализации проекта могут увеличиться.",
            ],
            opportunities=[
                "Усиление позиций на ключевом рынке.",
                "Повод для диалога о финансировании и хеджировании.",
            ],
            bank_position=(
                "Позиция банка: событие нейтрально-позитивно, но требует проверки "
                "долговой нагрузки и параметров сделки."
            ),
            ai_conclusion=(
                "Вывод AI: сигнал умеренно позитивный, эффект зависит от скорости "
                "исполнения и стоимости капитала."
            ),
            sources=sources,
            actions=[
                "Проверить финансовые показатели компании.",
                "Подготовить сценарий влияния на денежный поток.",
            ],
            confidence=0.72,
            meta={"model": "stub", "prompt_version": "v1"},
        )
        yield result(insight)


class LLMAnalyzer:
    def __init__(self, client: OpenAICompatClient, model: str) -> None:
        self.client = client
        self.model = model

    async def analyze(self, payload: AnalysisRequest) -> AsyncIterator[AgentEvent]:
        from app.pipeline.enrichment import enrich_evidence

        yield step("planning", "running", "Планируем анализ")
        yield step("planning", "done", "План готов")

        yield step("enriching", "running", "Подтягиваем RAG-контекст")
        before = len(payload.evidence)
        try:
            payload = await enrich_evidence(payload)
            added = len(payload.evidence) - before
            yield step("enriching", "done", f"Evidence: {len(payload.evidence)} (+{added})")
        except Exception as exc:
            print(f"[analyzer] enrichment failed: {exc!r}")
            yield step("enriching", "done", f"Evidence: {len(payload.evidence)} (без RAG)")

        yield step("analyzing", "running", "Модель формирует инсайт")
        insight, issues = await self._generate(payload)
        yield step("analyzing", "done", "Инсайт готов")

        yield step("validating", "running", "Проверяем источники и структуру")
        if issues:
            yield step("validating", "done", f"Замечаний: {len(issues)}")
        else:
            yield step("validating", "done", "Валидация пройдена")

        yield result(insight)

    async def _generate(self, payload: AnalysisRequest) -> tuple[Insight, list[ValidationIssue]]:
        from app.llm import budget
        from app.pipeline import cache

        cached = cache.get(payload)
        if cached is not None:
            print("[cache] hit")
            try:
                insight = self._to_insight(cached, payload)
                insight.meta["cache"] = "hit"
                return insight, []
            except Exception as exc:
                print(f"[cache] corrupt, ignore: {exc!r}")

        print("[cache] miss")
        last_raw: dict | None = None
        last_issues: list[ValidationIssue] = []

        for attempt in range(2):
            user = payload.model_dump_json()
            if attempt == 1 and last_raw is not None:
                user = build_repair_prompt(
                    json.dumps(last_raw, ensure_ascii=False),
                    format_issues(last_issues),
                )
            try:
                raw, usage = await self.client.complete_json(SYSTEM_PROMPT, user)
                last_raw = raw
                if usage:
                    raw.setdefault("meta", {})
                    raw["meta"]["usage"] = {
                        "input_tokens": usage.get("prompt_tokens"),
                        "output_tokens": usage.get("completion_tokens"),
                    }
                print(f"[agent] raw keys={list(raw.keys())}")
                insight = self._to_insight(raw, payload)
                issues = validate_insight(insight, payload)
                if not issues:
                    cache.put(payload, raw)
                    insight.meta["cache"] = "miss"
                    return insight, []
                last_issues = issues
                print(f"[agent] validation attempt {attempt + 1} failed: {format_issues(issues)}")
            except budget.BudgetExceeded as exc:
                print(f"[agent] budget exceeded: {exc}")
                last_issues = [ValidationIssue(code="budget_exceeded", message=str(exc))]
                break
            except Exception as exc:
                print(f"[agent] LLM attempt {attempt + 1} failed: {type(exc).__name__}: {exc!r}")
                last_issues = [
                    ValidationIssue(
                        code="llm_error",
                        message=f"{type(exc).__name__}: {exc!r}",
                    )
                ]

        insight = self._to_insight(last_raw or self._fallback(payload), payload)
        insight.meta = {
            **insight.meta,
            "validation_issues": [i.message for i in last_issues],
            "degraded": True,
            "cache": "miss",
        }
        return insight, last_issues

    def _to_insight(self, raw: dict, payload: AnalysisRequest | None = None) -> Insight:
        raw.setdefault("meta", {})
        raw["meta"].update({"model": self.model, "prompt_version": "v3"})

        # Автоматические подсказки для трассировки
        if payload is not None:
            from app.llm.sources import is_alfa_source

            alfa_sources = [e.source_id for e in payload.evidence if is_alfa_source(e.source_id)]
            if alfa_sources:
                raw["meta"]["alfa_analytics_sources"] = alfa_sources
            company = payload.company
            default_signals: list[str] = []
            if company.industry:
                default_signals.append(f"industry:{company.industry}")
            if company.region:
                default_signals.append(f"region:{company.region}")
            if default_signals and not raw["meta"].get("signals_used"):
                raw["meta"]["signals_used"] = default_signals

        # Подстраховка для legacy-полей
        if not raw.get("bank_position"):
            raw["bank_position"] = raw.get("bank_view", {}).get("text", "")
        if not raw.get("ai_conclusion"):
            raw["ai_conclusion"] = raw.get("ai_view", {}).get("text", "")
        if not raw.get("risks"):
            raw["risks"] = [c.get("text", "") for c in raw.get("structured_risks", [])]
        if not raw.get("opportunities"):
            raw["opportunities"] = [
                c.get("text", "") for c in raw.get("structured_opportunities", [])
            ]
        return Insight.model_validate(raw)

    def _fallback(self, payload: AnalysisRequest) -> dict:
        src = [{"source_id": e.source_id, "title": e.title, "url": e.url} for e in payload.evidence]
        return {
            "summary": f"{payload.company.name}: {payload.event.title}",
            "why_it_matters": "Не удалось получить ответ LLM; показан безопасный каркас.",
            "facts": [],
            "hypotheses": [
                {
                    "text": "Требуется повторный анализ: модель недоступна.",
                    "source_ids": [e.source_id for e in payload.evidence],
                    "confidence": "low",
                }
            ],
            "structured_risks": [],
            "structured_opportunities": [],
            "bank_position": "",
            "bank_view": {"used": False, "text": "", "source_ids": []},
            "ai_conclusion": "Вывод AI недоступен.",
            "ai_view": {"text": "", "source_ids": []},
            "risks": ["Анализ не завершён — источники не обработаны моделью."],
            "opportunities": [],
            "actions": ["Повторить запрос позже."],
            "sources": src,
            "confidence": 0.1,
            "disclaimer": "Не является инвестиционной рекомендацией",
            "meta": {"degraded": True},
        }


def create_analyzer(settings: Settings) -> Analyzer:
    if settings.agent_mode == "live" and settings.llm_base_url and settings.llm_model:
        client = OpenAICompatClient(
            base_url=settings.llm_base_url,
            api_key=settings.llm_api_key,
            model=settings.llm_model,
        )
        return LLMAnalyzer(client=client, model=settings.llm_model)
    return StubAnalyzer(delay=settings.step_delay())

import asyncio
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
        yield step("retrieving", "running", "Собираем контекст и источники")
        await asyncio.sleep(0.05)
        yield step("retrieving", "done", "Контекст собран")

        yield step("analyzing", "running", "Модель оценивает влияние события")
        insight = await self._generate(payload)
        yield step("analyzing", "done", "Вывод AI готов")
        yield result(insight)

    async def _generate(self, payload: AnalysisRequest) -> Insight:
        system = (
            "Ты аналитик крупного банка. Оцени влияние события на компанию. "
            "Верни только JSON по схеме Insight. Не придумывай источники: используй "
            "только source_id из evidence."
        )
        user = payload.model_dump_json()
        last_error: Exception | None = None

        for _ in range(2):
            try:
                raw = await self.client.complete_json(system, user)
                raw.setdefault("meta", {})
                raw["meta"].update({"model": self.model, "prompt_version": "v1"})
                return Insight.model_validate(raw)
            except Exception as exc:
                last_error = exc

        raise ValueError(f"agent failed to produce valid insight: {last_error}")


def create_analyzer(settings: Settings) -> Analyzer:
    if settings.agent_mode == "live" and settings.llm_base_url and settings.llm_model:
        client = OpenAICompatClient(
            base_url=settings.llm_base_url,
            api_key=settings.llm_api_key,
            model=settings.llm_model,
        )
        return LLMAnalyzer(client=client, model=settings.llm_model)
    return StubAnalyzer(delay=settings.step_delay())

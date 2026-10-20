from typing import Literal

from pydantic import BaseModel, ConfigDict, Field


class CompanyContext(BaseModel):
    model_config = ConfigDict(extra="ignore")

    id: str
    name: str
    inn: str = ""
    industry: str = ""
    region: str = ""
    description: str = ""


class EventContext(BaseModel):
    model_config = ConfigDict(extra="ignore")

    id: str
    title: str
    summary: str = ""
    published_at: str | None = None


class Evidence(BaseModel):
    model_config = ConfigDict(extra="ignore")

    source_id: str
    title: str
    url: str
    snippet: str = ""
    published_at: str | None = None


class AnalysisRequest(BaseModel):
    model_config = ConfigDict(extra="ignore")

    request_id: str
    company: CompanyContext
    event: EventContext
    evidence: list[Evidence] = Field(default_factory=list)
    locale: str = "ru"
    mode: Literal["live", "demo"] = "live"


class Source(BaseModel):
    source_id: str
    title: str
    url: str


class Insight(BaseModel):
    summary: str
    why_it_matters: str
    risks: list[str]
    opportunities: list[str]
    bank_position: str
    ai_conclusion: str
    sources: list[Source]
    actions: list[str]
    confidence: float = Field(ge=0, le=1)
    meta: dict[str, object] = Field(default_factory=dict)


class StepEvent(BaseModel):
    type: Literal["step"] = "step"
    name: str
    status: Literal["running", "done"]
    message: str = ""


class ResultEvent(BaseModel):
    type: Literal["result"] = "result"
    insight: Insight
    meta: dict[str, object] = Field(default_factory=dict)


class ErrorEvent(BaseModel):
    type: Literal["error"] = "error"
    code: str
    message: str
    retryable: bool = False

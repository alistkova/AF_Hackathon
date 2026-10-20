import asyncio
import logging

from fastapi import APIRouter, Request
from sse_starlette.sse import EventSourceResponse

from app.contracts.models import AnalysisRequest, ErrorEvent

router = APIRouter()
logger = logging.getLogger(__name__)


@router.post("/v1/analyses")
async def create_analysis(payload: AnalysisRequest, request: Request) -> EventSourceResponse:
    analyzer = request.app.state.analyzer

    async def stream():
        try:
            async for event in analyzer.analyze(payload):
                yield event
        except asyncio.CancelledError:
            raise
        except Exception as exc:
            logger.exception("analysis failed", extra={"request_id": payload.request_id})
            error = ErrorEvent(
                code="analysis_failed",
                message=str(exc),
                retryable=True,
            )
            yield {"event": "error", "data": error.model_dump_json()}

    return EventSourceResponse(stream(), ping=15)

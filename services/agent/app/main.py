import logging

from fastapi import FastAPI

from app.api.routes import router
from app.config import get_settings
from app.pipeline.analyzer import create_analyzer

logging.basicConfig(level=logging.INFO)

settings = get_settings()

app = FastAPI(title="Alfa Hack Agent", version="0.1.0")
app.state.analyzer = create_analyzer(settings)
app.include_router(router)


@app.get("/healthz")
async def healthz() -> dict[str, str]:
    return {"status": "ok", "mode": settings.agent_mode}

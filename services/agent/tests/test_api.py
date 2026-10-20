import json

import httpx
import pytest

from app.main import app
from app.pipeline.analyzer import StubAnalyzer


def request_body() -> dict[str, object]:
    return {
        "request_id": "test-job",
        "company": {"id": "magnit", "name": "Магнит"},
        "event": {"id": "news-001", "title": "Магнит расширяет логистику"},
        "evidence": [
            {
                "source_id": "news-001",
                "title": "Магнит расширяет логистику",
                "url": "https://example.com/news-001",
            }
        ],
        "locale": "ru",
        "mode": "demo",
    }


@pytest.mark.asyncio
async def test_health() -> None:
    transport = httpx.ASGITransport(app=app)
    async with httpx.AsyncClient(transport=transport, base_url="http://test") as client:
        response = await client.get("/healthz")
    assert response.status_code == 200
    assert response.json()["status"] == "ok"


@pytest.mark.asyncio
async def test_analysis_stream() -> None:
    app.state.analyzer = StubAnalyzer(delay=0)

    transport = httpx.ASGITransport(app=app)
    async with httpx.AsyncClient(transport=transport, base_url="http://test") as client:
        response = await client.post("/v1/analyses", json=request_body())

    assert response.status_code == 200
    assert "event: step" in response.text
    assert "event: result" in response.text

    data_lines = [
        line.removeprefix("data: ")
        for line in response.text.splitlines()
        if line.startswith("data: ")
    ]
    result = json.loads(data_lines[-1])
    assert result["type"] == "result"
    assert result["insight"]["sources"][0]["source_id"] == "news-001"

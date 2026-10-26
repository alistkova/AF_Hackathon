#!/usr/bin/env python3
"""Прогоняет eval-набор через агента и считает метрики.

Usage:
  uv run python scripts/eval_agent.py            # live LLM
  uv run python scripts/eval_agent.py --stub     # stub (без токенов, для отладки)
  uv run python scripts/eval_agent.py --id 01_rate_magnit_alfa
"""
from __future__ import annotations

import argparse
import asyncio
import json
import sys
import time
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
FIXTURES = ROOT / "tests" / "fixtures" / "eval"
REPORTS = ROOT / "reports"
REPORTS.mkdir(exist_ok=True)

sys.path.insert(0, str(ROOT))

from app.config import get_settings  # noqa: E402
from app.contracts.models import AnalysisRequest, ResultEvent  # noqa: E402
from app.pipeline.analyzer import LLMAnalyzer, StubAnalyzer  # noqa: E402
from app.pipeline.metrics import (  # noqa: E402
    aggregate,
    check_thresholds,
    evaluate_event,
)


async def run_one(analyzer, payload: AnalysisRequest) -> dict:
    started = time.time()
    insight = None
    async for event in analyzer.analyze(payload):
        if event.get("event") == "result":
            data = json.loads(event["data"])
            insight = ResultEvent.model_validate(data).insight
            break
    latency = time.time() - started
    if insight is None:
        return {
            "id": payload.request_id,
            "ok": False,
            "error": "no result event",
            "latency_s": round(latency, 2),
            "issues": ["no_result"],
        }
    m = evaluate_event(insight, payload)
    return {
        "id": payload.request_id,
        "ok": not m.issues,
        "latency_s": round(latency, 2),
        "source_coverage": round(m.source_coverage, 3),
        "bank_view_used": m.bank_view_used,
        "bank_view_expected": m.bank_view_expected,
        "separation_ok": m.separation_ok,
        "degraded": m.degraded,
        "issues": m.issues,
        "_metrics": m,
    }


def load_fixtures(only: str | None = None) -> list[dict]:
    files = sorted(FIXTURES.glob("*.json"))
    if only:
        files = [f for f in files if only in f.name]
    out = []
    for f in files:
        data = json.loads(f.read_text(encoding="utf-8"))
        out.append({"id": data.get("id", f.stem), "payload": data["payload"]})
    return out


async def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--stub", action="store_true", help="использовать StubAnalyzer (без токенов)"
    )
    parser.add_argument(
        "--id", default=None, help="только одна фикстура (по подстроке в имени файла)"
    )
    args = parser.parse_args()

    settings = get_settings()
    if args.stub:
        analyzer = StubAnalyzer(delay=0)
        mode = "stub"
    else:
        if not settings.llm_base_url or not settings.llm_api_key:
            print(
                "ERROR: LLM_BASE_URL/LLM_API_KEY не заданы. "
                "Запусти с --stub или поправь .env",
                file=sys.stderr,
            )
            return 2
        from app.llm.openai_compat import OpenAICompatClient
        client = OpenAICompatClient(
            base_url=settings.llm_base_url,
            api_key=settings.llm_api_key,
            model=settings.llm_model,
        )
        analyzer = LLMAnalyzer(client=client, model=settings.llm_model)
        mode = "live"

    fixtures = load_fixtures(args.id)
    if not fixtures:
        print("ERROR: fixtures not found in", FIXTURES, file=sys.stderr)
        return 2

    print(f"running {len(fixtures)} fixtures, mode={mode}")
    per_event: list[dict] = []
    metric_rows = []
    for fixture in fixtures:
        payload = AnalysisRequest.model_validate(fixture["payload"])
        payload = payload.model_copy(update={"request_id": fixture["id"]})
        print(f"  -> {fixture['id']} ...", end="", flush=True)
        row = await run_one(analyzer, payload)
        metric_rows.append(row.pop("_metrics"))
        per_event.append(row)
        status = "OK" if row["ok"] else "ISSUES: " + ",".join(row["issues"])
        print(f" {row['latency_s']}s {status}")

    agg = aggregate(metric_rows)
    failures = check_thresholds(agg)

    report = {
        "run_at": time.strftime("%Y-%m-%dT%H:%M:%S"),
        "mode": mode,
        "model": settings.llm_model if mode == "live" else "stub",
        "aggregate": agg,
        "thresholds_failed": failures,
        "per_event": per_event,
    }
    path = REPORTS / f"eval_{time.strftime('%Y%m%d_%H%M%S')}.json"
    path.write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")

    print()
    print("aggregate:", json.dumps(agg, indent=2, ensure_ascii=False))
    if failures:
        print("THRESHOLDS FAILED:")
        for f in failures:
            print(" -", f)
        print(f"report saved: {path}")
        return 1
    print(f"all thresholds passed. report: {path}")
    return 0


if __name__ == "__main__":
    sys.exit(asyncio.run(main()))

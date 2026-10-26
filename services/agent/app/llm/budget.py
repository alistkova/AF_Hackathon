"""Дневной бюджет на LLM-вызовы. Счётчик хранится в JSON рядом с проектом."""
from __future__ import annotations

import json
import os
from datetime import datetime, timedelta, timezone
from pathlib import Path

# Оценка стоимости deepseek-v4.1-flash (примерная, $/1M токенов).
# При изменении модели — поправить здесь.
PRICE_INPUT_USD_PER_M = 0.14
PRICE_OUTPUT_USD_PER_M = 0.28
USD_TO_RUB = 90.0
DAILY_BUDGET_RUB = 450.0  # ниже 500, чтобы был запас

_STATE_DIR = Path(os.getenv("ALFA_AGENT_STATE_DIR", "~/.alfa_agent")).expanduser()
_STATE_FILE = _STATE_DIR / "usage.json"

# МСК = UTC+3, сброс бюджета в 3:00 МСК = 00:00 UTC
_MSK = timezone(timedelta(hours=3))


def _today() -> str:
    return datetime.now(_MSK).date().isoformat()


def _load() -> dict:
    if not _STATE_FILE.exists():
        return {}
    try:
        return json.loads(_STATE_FILE.read_text(encoding="utf-8"))
    except Exception:
        return {}


def _save(data: dict) -> None:
    _STATE_DIR.mkdir(parents=True, exist_ok=True)
    _STATE_FILE.write_text(json.dumps(data, ensure_ascii=False, indent=2), encoding="utf-8")


def cost_rub(input_tokens: int, output_tokens: int) -> float:
    usd = (
        input_tokens / 1_000_000 * PRICE_INPUT_USD_PER_M
        + output_tokens / 1_000_000 * PRICE_OUTPUT_USD_PER_M
    )
    return round(usd * USD_TO_RUB, 4)


def today_usage() -> dict:
    data = _load()
    return data.get(_today(), {"input_tokens": 0, "output_tokens": 0, "cost_rub": 0.0, "calls": 0})


def add_usage(input_tokens: int, output_tokens: int) -> dict:
    data = _load()
    day = _today()
    entry = data.setdefault(
        day,
        {"input_tokens": 0, "output_tokens": 0, "cost_rub": 0.0, "calls": 0},
    )
    entry["input_tokens"] += int(input_tokens)
    entry["output_tokens"] += int(output_tokens)
    entry["cost_rub"] = round(entry["cost_rub"] + cost_rub(input_tokens, output_tokens), 4)
    entry["calls"] += 1
    _save(data)
    return entry


def is_exceeded() -> bool:
    return today_usage()["cost_rub"] >= DAILY_BUDGET_RUB


class BudgetExceeded(RuntimeError):
    pass


def check() -> None:
    if is_exceeded():
        raise BudgetExceeded(
            f"daily budget exceeded: {today_usage()['cost_rub']} / {DAILY_BUDGET_RUB} RUB"
        )

"""Простой in-memory кеш инсайтов с TTL."""
from __future__ import annotations

import hashlib
import time
from dataclasses import dataclass

from app.contracts.models import AnalysisRequest

TTL_SECONDS = 3600


@dataclass
class CacheEntry:
    insight: dict
    created_at: float


_store: dict[str, CacheEntry] = {}


def make_key(payload: AnalysisRequest) -> str:
    ids = sorted(e.source_id for e in payload.evidence)
    raw = "|".join([
        payload.company.id,
        payload.event.id,
        ",".join(ids),
        payload.locale,
    ])
    return hashlib.sha256(raw.encode("utf-8")).hexdigest()


def get(payload: AnalysisRequest) -> dict | None:
    key = make_key(payload)
    entry = _store.get(key)
    if entry is None:
        return None
    if time.time() - entry.created_at > TTL_SECONDS:
        _store.pop(key, None)
        return None
    return entry.insight


def put(payload: AnalysisRequest, insight: dict) -> None:
    _store[make_key(payload)] = CacheEntry(insight=insight, created_at=time.time())


def clear() -> None:
    _store.clear()


def size() -> int:
    return len(_store)

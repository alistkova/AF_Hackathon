"""RAG-обогащение evidence: если контекста мало, подтягиваем chunks из content service."""
from __future__ import annotations

import httpx

from app.config import get_settings
from app.contracts.models import AnalysisRequest, Evidence
from app.rag_system.pipeline import list_chunks, search_chunks

MAX_EVIDENCE = 8
MIN_EVIDENCE = 3
SNIPPET_LIMIT = 500
NEWS_FALLBACK_LIMIT = 5


def _chunk_to_evidence(chunk: dict) -> Evidence | None:
    source = chunk.get("source") or "unknown"
    page = chunk.get("page") or 0
    number = chunk.get("chunk") or 0
    content = (chunk.get("content") or "").strip()
    if not content:
        return None
    return Evidence(
        source_id=f"rag:{source}:{page}:{number}",
        title=f"{source} (page {page})",
        url=f"rag://{source}#page={page}&chunk={number}",
        snippet=content[:SNIPPET_LIMIT],
        published_at=chunk.get("published_at"),
    )


def _news_to_evidence(item: dict) -> Evidence | None:
    sid = item.get("document_id") or item.get("news_id")
    if not sid:
        return None
    content = (item.get("full_text") or item.get("summary") or "").strip()
    if not content:
        return None
    return Evidence(
        source_id=f"news:{sid}",
        title=item.get("title") or "",
        url=item.get("source_url") or item.get("url") or "",
        snippet=content[:SNIPPET_LIMIT],
        published_at=item.get("published_at"),
    )


def _industry_to_sector(industry: str) -> str:
    if not industry:
        return ""
    mapping = {
        "ритейл": "retail", "розниц": "retail",
        "металлург": "metallurgy", "сталь": "metallurgy",
        "нефт": "oil_gas", "газ": "oil_gas",
        "банк": "banking", "финанс": "banking",
        "ит": "it", "технолог": "it",
        "телеком": "telecom",
        "сельхоз": "agriculture", "агро": "agriculture",
        "добыч": "mining", "шахт": "mining",
        "энергетик": "energy",
    }
    low = industry.lower()
    for key, sector in mapping.items():
        if key in low:
            return sector
    return ""


async def _news_fallback(payload: AnalysisRequest) -> list[Evidence]:
    """Если RAG пуст — тянем новости из content service по компании/сектору."""
    s = get_settings()
    url = f"{s.content_url}/api/v1/news"
    params: dict[str, object] = {"kind": "news", "limit": NEWS_FALLBACK_LIMIT}
    sector = _industry_to_sector(payload.company.industry)
    if sector:
        params["sector"] = sector
    try:
        async with httpx.AsyncClient(timeout=10) as client:
            r = await client.get(url, params=params)
            r.raise_for_status()
            data = r.json()
            if isinstance(data, list):
                items = data
            elif isinstance(data, dict):
                items = data.get("items") or data.get("news") or []
            else:
                items = []
    except Exception as exc:
        print(f"[enrichment] news fallback failed: {exc!r}")
        return []

    result: list[Evidence] = []
    for item in items[:NEWS_FALLBACK_LIMIT]:
        if not isinstance(item, dict):
            continue
        ev = _news_to_evidence(item)
        if ev is not None:
            result.append(ev)
    print(f"[enrichment] news fallback: {len(result)} items (sector={sector or 'any'})")
    return result


async def enrich_evidence(payload: AnalysisRequest) -> AnalysisRequest:
    """Возвращает копию payload с расширенным evidence. Не мутирует вход."""
    if len(payload.evidence) >= MIN_EVIDENCE:
        return payload

    gathered: list[Evidence] = list(payload.evidence)
    seen: set[str] = {e.source_id for e in gathered}

    def add_ev(ev: Evidence | None) -> None:
        if ev is None or ev.source_id in seen:
            return
        if len(gathered) >= MAX_EVIDENCE:
            return
        gathered.append(ev)
        seen.add(ev.source_id)

    def add_chunks(items: list[dict]) -> None:
        for chunk in items:
            if len(gathered) >= MAX_EVIDENCE:
                return
            add_ev(_chunk_to_evidence(chunk))

    def add_evidence(items: list[Evidence]) -> None:
        for ev in items:
            if len(gathered) >= MAX_EVIDENCE:
                return
            add_ev(ev)

    # 1. Semantic search по компании
    try:
        search = search_chunks(
            query=payload.event.title,
            k=5,
            company_id=payload.company.id,
        )
        add_chunks(search.get("chunks", []))
    except Exception as exc:
        print(f"[enrichment] search_chunks (company) failed: {exc!r}")

    # 2. Fallback: global semantic search без фильтра по компании
    if len(gathered) < MIN_EVIDENCE:
        try:
            search = search_chunks(query=payload.event.title, k=5)
            add_chunks(search.get("chunks", []))
        except Exception as exc:
            print(f"[enrichment] search_chunks (global) failed: {exc!r}")

    # 3. Fallback: list chunks по компании (без семантики)
    if len(gathered) < MIN_EVIDENCE:
        try:
            listing = list_chunks(company_id=payload.company.id, k=3)
            add_chunks(listing.get("chunks", []))
        except Exception as exc:
            print(f"[enrichment] list_chunks failed: {exc!r}")

    # 4. Fallback: новости из content service по отрасли
    if len(gathered) < MIN_EVIDENCE:
        add_evidence(await _news_fallback(payload))

    print(f"[enrichment] evidence {len(payload.evidence)} -> {len(gathered)}")
    return payload.model_copy(update={"evidence": gathered})

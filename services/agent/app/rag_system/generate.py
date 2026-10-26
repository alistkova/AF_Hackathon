import json
import time
from typing import Any

import requests

from .config import DEFAULT_MODEL, OLLAMA_BASE_URL, OLLAMA_MODELS
from .retrieve import (
    format_context,
    retrieve_with_filters,
    source_labels,
)


def _record(
    source: str,
    model: str,
    elapsed: float,
    ok: bool,
    error: str | None,
    usage: dict[str, Any],
) -> None:
    from app.metrics.recorder import recorder

    in_tokens = int(usage.get("prompt_eval_count")) if usage.get("prompt_eval_count") else 0
    out_tokens = int(usage.get("eval_count")) if usage.get("eval_count") else 0
    total = in_tokens + out_tokens
    recorder().record(
        source=source,
        model=model,
        e2el_s=elapsed,
        input_tokens=in_tokens,
        output_tokens=out_tokens,
        total_tokens=total,
        output_tput=out_tokens / elapsed if elapsed > 0 else 0.0,
        total_tput=total / elapsed if elapsed > 0 else 0.0,
        ok=ok,
        error=error,
    )


def generate_with_ollama(
    prompt: str,
    model: str = DEFAULT_MODEL,
    temperature: float = 0.3,
    max_tokens: int = 2048,
    system_prompt: str | None = None,
) -> str:
    """Generate response using Ollama API."""
    model_name = OLLAMA_MODELS.get(model, model)

    messages = []
    if system_prompt:
        messages.append({"role": "system", "content": system_prompt})
    messages.append({"role": "user", "content": prompt})

    payload = {
        "model": model_name,
        "messages": messages,
        "temperature": temperature,
        "max_tokens": max_tokens,
        "stream": False,
    }

    start = time.perf_counter()
    try:
        response = requests.post(f"{OLLAMA_BASE_URL}/api/chat", json=payload, timeout=120)
        response.raise_for_status()
        data = response.json()
        _record("rag.query", model_name, time.perf_counter() - start, True, None, data)
        return data.get("message", {}).get("content", "")
    except requests.exceptions.ConnectionError:
        _record("rag.query", model_name, time.perf_counter() - start, False, "connection_error", {})
        return f"Error: Cannot connect to Ollama at {OLLAMA_BASE_URL}. Is Ollama running?"
    except Exception as e:
        _record("rag.query", model_name, time.perf_counter() - start, False, str(e), {})
        return f"Error generating response: {e}"


RAG_SYSTEM_PROMPT = (
    "Ты — эксперт по анализу новостей и финансовых отчетов компании АЛЬФА-Банк "
    "и смежных отраслей (сталь, металлургия, сырьевые рынки).\n\n"
    "Твоя задача: на основе предоставленного контекста из новостей и отчетов "
    "дать качественный инсайт, вывод или ответ на вопрос пользователя.\n\n"
    "Правила:\n"
    "1. Используй ТОЛЬКО информацию из предоставленного контекста\n"
    "2. Если в контексте нет ответа — честно скажи об этом\n"
    "3. Цитируй источники в формате [Source N]\n"
    "4. Будь лаконичен, но глубок в анализе\n"
    "5. Выделяй ключевые метрики, тренды, риски и возможности\n"
    "6. Отвечай на русском языке"
)


def rag_query(
    question: str,
    model: str = DEFAULT_MODEL,
    k: int = 5,
    temperature: float = 0.3,
    company_id: str | None = None,
    sector: str | None = None,
    published_after: str | None = None,
    published_before: str | None = None,
) -> dict[str, Any]:
    """Full RAG pipeline with optional metadata filters: retrieve + generate."""
    docs = retrieve_with_filters(
        question,
        k=k,
        company_id=company_id,
        sector=sector,
        published_after=published_after,
        published_before=published_before,
    )
    context = format_context(docs)

    prompt = f"""Контекст из новостей и отчетов:
{context}

Вопрос: {question}

Дай подробный ответ с инсайтами, опираясь только на контекст выше."""

    answer = generate_with_ollama(
        prompt=prompt, model=model, temperature=temperature, system_prompt=RAG_SYSTEM_PROMPT
    )

    return {
        "question": question,
        "answer": answer,
        "sources": source_labels(docs),
        "context_used": context,
        "model": model,
    }


def rag_stream(
    question: str,
    model: str = DEFAULT_MODEL,
    k: int = 5,
    temperature: float = 0.3,
    company_id: str | None = None,
    sector: str | None = None,
    published_after: str | None = None,
    published_before: str | None = None,
):
    """Stream RAG response (generator)."""
    docs = retrieve_with_filters(
        question,
        k=k,
        company_id=company_id,
        sector=sector,
        published_after=published_after,
        published_before=published_before,
    )
    context = format_context(docs)
    model_name = OLLAMA_MODELS.get(model, model)

    prompt = f"""Контекст из новостей и отчетов:
{context}

Вопрос: {question}

Дай подробный ответ с инсайтами, опираясь только на контекст выше."""

    messages = [
        {"role": "system", "content": RAG_SYSTEM_PROMPT},
        {"role": "user", "content": prompt},
    ]

    payload = {
        "model": model_name,
        "messages": messages,
        "temperature": temperature,
        "max_tokens": 2048,
        "stream": True,
    }

    try:
        start = time.perf_counter()
        with requests.post(
            f"{OLLAMA_BASE_URL}/api/chat", json=payload, stream=True, timeout=120
        ) as response:
            response.raise_for_status()
            done: dict[str, Any] = {}
            for line in response.iter_lines():
                if line:
                    data = json.loads(line.decode("utf-8"))
                    if "message" in data and "content" in data["message"]:
                        yield data["message"]["content"]
                    if data.get("done", False):
                        done = data
                        break
        _record("rag.stream", model_name, time.perf_counter() - start, True, None, done)
    except requests.exceptions.ConnectionError:
        _record("rag.stream", model_name, time.perf_counter() - start, False, "connection_error", {})
        yield f"Error: Cannot connect to Ollama at {OLLAMA_BASE_URL}"
    except Exception as e:
        _record("rag.stream", model_name, time.perf_counter() - start, False, str(e), {})
        yield f"Error: {e}"

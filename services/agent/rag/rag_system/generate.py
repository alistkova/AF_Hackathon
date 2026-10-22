import os
import json
from typing import Optional, List, Dict, Any
import requests

from .config import OLLAMA_BASE_URL, OLLAMA_MODELS
from .retrieve import retrieve_with_context


def generate_with_ollama(
    prompt: str,
    model: str = "qwen",
    temperature: float = 0.3,
    max_tokens: int = 2048,
    system_prompt: Optional[str] = None,
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

    try:
        response = requests.post(f"{OLLAMA_BASE_URL}/api/chat", json=payload, timeout=120)
        response.raise_for_status()
        data = response.json()
        return data.get("message", {}).get("content", "")
    except requests.exceptions.ConnectionError:
        return f"Error: Cannot connect to Ollama at {OLLAMA_BASE_URL}. Is Ollama running?"
    except Exception as e:
        return f"Error generating response: {e}"


RAG_SYSTEM_PROMPT = """Ты — эксперт по анализу новостей и финансовых отчетов компании АЛЬФА-Банк и смежных отраслей (сталь, металлургия, сырьевые рынки).

Твоя задача: на основе предоставленного контекста из новостей и отчетов дать качественный инсайт, вывод или ответ на вопрос пользователя.

Правила:
1. Используй ТОЛЬКО информацию из предоставленного контекста
2. Если в контексте нет ответа — честно скажи об этом
3. Цитируй источники в формате [Source N]
4. Будь лаконичен, но глубок в анализе
5. Выделяй ключевые метрики, тренды, риски и возможности
6. Отвечай на русском языке"""


def rag_query(
    question: str, model: str = "qwen", k: int = 5, temperature: float = 0.3
) -> Dict[str, Any]:
    """Full RAG pipeline: retrieve + generate."""
    context = retrieve_with_context(question, k=k)

    prompt = f"""Контекст из новостей и отчетов:
{context}

Вопрос: {question}

Дай подробный ответ с инсайтами, опираясь только на контекст выше."""

    answer = generate_with_ollama(
        prompt=prompt, model=model, temperature=temperature, system_prompt=RAG_SYSTEM_PROMPT
    )

    return {"question": question, "answer": answer, "context_used": context, "model": model}


def rag_stream(question: str, model: str = "qwen", k: int = 5, temperature: float = 0.3):
    """Stream RAG response (generator)."""
    context = retrieve_with_context(question, k=k)
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
        with requests.post(
            f"{OLLAMA_BASE_URL}/api/chat", json=payload, stream=True, timeout=120
        ) as response:
            response.raise_for_status()
            for line in response.iter_lines():
                if line:
                    data = json.loads(line.decode("utf-8"))
                    if "message" in data and "content" in data["message"]:
                        yield data["message"]["content"]
                    if data.get("done", False):
                        break
    except requests.exceptions.ConnectionError:
        yield f"Error: Cannot connect to Ollama at {OLLAMA_BASE_URL}"
    except Exception as e:
        yield f"Error: {e}"

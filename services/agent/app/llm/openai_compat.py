import json
import logging

import httpx

logger = logging.getLogger(__name__)

_FALLBACK_BASE_URL = "https://api.alfahack-gateway26.ru/v1"


class OpenAICompatClient:
    def __init__(self, base_url: str, api_key: str, model: str) -> None:
        self.base_url = base_url.rstrip("/")
        self.api_key = api_key
        self.model = model

    async def complete_json(self, system: str, user: str) -> dict[str, object]:
        # Без response_format: гейтвеи по-разному его поддерживают,
        # а на fallback он ломает ответ (возвращает {"type":"json_object"}).
        payload = {
            "model": self.model,
            "temperature": 0.2,
            "messages": [
                {"role": "system", "content": system},
                {"role": "user", "content": user},
            ],
        }
        headers = {"Authorization": f"Bearer {self.api_key}"}

        urls = [self.base_url]
        if _FALLBACK_BASE_URL not in urls:
            urls.append(_FALLBACK_BASE_URL)

        last_exc: Exception | None = None
        for base in urls:
            try:
                async with httpx.AsyncClient(timeout=60) as client:
                    response = await client.post(
                        f"{base}/chat/completions",
                        json=payload,
                        headers=headers,
                    )
                    response.raise_for_status()
                    body = response.json()
                content = body["choices"][0]["message"]["content"]
                logger.info("[llm] ok via %s model=%s", base, self.model)
                print(f"[llm] raw content preview: {str(content)[:200]!r}")
                return self._parse_json(content)
            except (httpx.ConnectError, httpx.ConnectTimeout) as exc:
                logger.warning("[llm] %s unreachable: %r", base, exc)
                last_exc = exc
                continue
            except Exception as exc:
                logger.warning("[llm] %s failed: %r", base, exc)
                last_exc = exc
                continue

        raise RuntimeError(f"all LLM endpoints failed: {last_exc!r}") from last_exc

    @staticmethod
    def _parse_json(content: object) -> dict[str, object]:
        if isinstance(content, dict):
            return content
        if not isinstance(content, str):
            raise ValueError(f"unexpected content type: {type(content)!r}")

        text = content.strip()
        # Снимаем markdown-фенсы ```json ... ```
        if text.startswith("```"):
            text = text.strip("`")
            if text.lower().startswith("json"):
                text = text[4:]
            text = text.strip()
        # На всякий случай — найти первую { и последнюю }
        if not text.startswith("{"):
            start = text.find("{")
            end = text.rfind("}")
            if start == -1 or end == -1:
                raise ValueError(f"no JSON object in content: {content!r}")
            text = text[start : end + 1]
        return json.loads(text)

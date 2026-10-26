import json
import logging
import time

import httpx

from app.llm import budget
from app.metrics.recorder import recorder

logger = logging.getLogger(__name__)

_FALLBACK_BASE_URL = "https://api.alfahack-gateway26.ru/v1"


class OpenAICompatClient:
    def __init__(
        self,
        base_url: str,
        api_key: str,
        model: str,
        metric_source: str = "agent.analyze",
    ) -> None:
        self.base_url = base_url.rstrip("/")
        self.api_key = api_key
        self.model = model
        self.metric_source = metric_source

    async def complete_json(self, system: str, user: str) -> tuple[dict, dict | None]:
        budget.check()

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
        start = time.perf_counter()
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
                elapsed = time.perf_counter() - start
                content = body["choices"][0]["message"]["content"]
                usage = body.get("usage") or {}
                if usage:
                    budget.add_usage(
                        usage.get("prompt_tokens", 0),
                        usage.get("completion_tokens", 0),
                    )
                    logger.info(
                        "[llm] via %s in=%s out=%s cost≈%s₽",
                        base,
                        usage.get("prompt_tokens"),
                        usage.get("completion_tokens"),
                        budget.cost_rub(
                            usage.get("prompt_tokens", 0),
                            usage.get("completion_tokens", 0),
                        ),
                    )
                else:
                    logger.info("[llm] via %s (no usage in response)", base)
                self._record(elapsed, usage, ok=True, error=None)
                print(f"[llm] raw content preview: {str(content)[:200]!r}")
                return self._parse_json(content), usage
            except budget.BudgetExceeded:
                raise
            except (httpx.ConnectError, httpx.ConnectTimeout) as exc:
                logger.warning("[llm] %s unreachable: %r", base, exc)
                last_exc = exc
                continue
            except Exception as exc:
                logger.warning("[llm] %s failed: %r", base, exc)
                last_exc = exc
                continue

        self._record(time.perf_counter() - start, {}, ok=False, error=repr(last_exc))
        raise RuntimeError(f"all LLM endpoints failed: {last_exc!r}") from last_exc

    def _record(self, elapsed: float, usage: dict, ok: bool, error: str | None) -> None:
        in_tokens = int(usage.get("prompt_tokens") or 0)
        out_tokens = int(usage.get("completion_tokens") or 0)
        total = in_tokens + out_tokens
        recorder().record(
            source=self.metric_source,
            model=self.model,
            e2el_s=elapsed,
            input_tokens=in_tokens,
            output_tokens=out_tokens,
            total_tokens=total,
            output_tput=out_tokens / elapsed if elapsed > 0 else 0.0,
            total_tput=total / elapsed if elapsed > 0 else 0.0,
            ok=ok,
            error=error,
        )

    @staticmethod
    def _parse_json(content: object) -> dict:
        if isinstance(content, dict):
            return content
        if not isinstance(content, str):
            raise ValueError(f"unexpected content type: {type(content)!r}")

        text = content.strip()
        if text.startswith("```"):
            text = text.strip("`")
            if text.lower().startswith("json"):
                text = text[4:]
            text = text.strip()
        if not text.startswith("{"):
            start = text.find("{")
            end = text.rfind("}")
            if start == -1 or end == -1:
                raise ValueError(f"no JSON object in content: {content!r}")
            text = text[start : end + 1]
        return json.loads(text)

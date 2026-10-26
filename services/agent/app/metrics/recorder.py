import json
import logging
import threading
from datetime import UTC, datetime
from pathlib import Path

from app.config import get_settings

logger = logging.getLogger(__name__)

_FILENAME = "llm_metrics.jsonl"
_instance: "MetricsRecorder | None" = None
_instance_lock = threading.Lock()


class MetricsRecorder:
    def __init__(self, directory: str | Path, enabled: bool = True) -> None:
        self.directory = Path(directory)
        self.enabled = enabled
        self._write_lock = threading.Lock()
        self._target = self.directory / _FILENAME if enabled else None

    def record(self, **fields: object) -> None:
        if not self.enabled:
            return
        entry = {"ts": datetime.now(UTC).isoformat(), **fields}
        with self._write_lock:
            try:
                self.directory.mkdir(parents=True, exist_ok=True)
                with open(self._target, "a", encoding="utf-8") as handle:
                    handle.write(json.dumps(entry, ensure_ascii=False, default=str) + "\n")
            except Exception:
                logger.warning("metrics write failed", exc_info=True)
        logger.info("metric %s", json.dumps(entry, ensure_ascii=False, default=str))


def recorder() -> MetricsRecorder:
    global _instance
    with _instance_lock:
        if _instance is None:
            settings = get_settings()
            _instance = MetricsRecorder(
                directory=settings.metrics_dir,
                enabled=settings.metrics_enabled,
            )
    return _instance

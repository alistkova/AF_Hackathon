from functools import lru_cache
from pathlib import Path
from tempfile import gettempdir
from typing import Literal

from pydantic_settings import BaseSettings, SettingsConfigDict

_AGENT_DIR = Path(__file__).resolve().parent.parent
_TEMP_DIR = Path(gettempdir())


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", extra="ignore")

    agent_mode: Literal["stub", "live"] = "stub"
    agent_step_delay: str = "350ms"

    llm_base_url: str = ""
    llm_fallback_base_url: str = ""
    llm_api_key: str = ""
    llm_model: str = ""

    rag_index_dir: str = str(_TEMP_DIR / "alfa_rag_index")
    rag_data_dir: str = str(_AGENT_DIR / "data")
    rag_use_ocr: bool = False
    content_url: str = "http://localhost:8082"

    metrics_enabled: bool = True
    metrics_dir: str = str(_AGENT_DIR / "metrics")

    def step_delay(self) -> float:
        raw = self.agent_step_delay.strip().lower()
        if raw.endswith("ms"):
            return float(raw[:-2]) / 1000
        if raw.endswith("s"):
            return float(raw[:-1])
        return float(raw)


@lru_cache
def get_settings() -> Settings:
    return Settings()

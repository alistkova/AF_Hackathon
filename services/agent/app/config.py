from functools import lru_cache
from typing import Literal

from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", extra="ignore")

    agent_mode: Literal["stub", "live"] = "stub"
    agent_step_delay: str = "350ms"

    llm_base_url: str = ""
    llm_api_key: str = ""
    llm_model: str = ""

    # RAG settings
    rag_index_dir: str = "C:/temp/alfa_rag_index"
    rag_data_dir: str = "services/agent/rag/data"
    rag_use_ocr: bool = False
    rag_tfidf_max_features: int = 5000

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

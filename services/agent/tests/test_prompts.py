from app.llm.prompts import SYSTEM_PROMPT
from app.llm.sources import is_alfa_source


def test_alfa_detection():
    assert is_alfa_source("rag:ALFA-Macro_Insights-Policy_Rate_11Sep.pdf:1:0")
    assert is_alfa_source("rag:FI_Strategy_3Q26_22072026.pdf:21:1")
    assert is_alfa_source("rag:Inflation_tracker_02.pdf:1:0")
    assert is_alfa_source("rag:Gold_digest_August26.pdf:1:0")
    assert is_alfa_source("rag:Бонд_Бриф_14092026.pdf:5:0")
    assert not is_alfa_source("n1")
    assert not is_alfa_source("rag:news.pdf:1:0")
    assert not is_alfa_source("rag:unknown.pdf:2:0")


def test_prompt_mentions_bank_view_rule():
    assert "bank_view.used = true" in SYSTEM_PROMPT
    assert "ALFA-" in SYSTEM_PROMPT


def test_prompt_mentions_signals_used():
    assert "meta.signals_used" in SYSTEM_PROMPT


def test_prompt_has_few_shot_examples():
    assert SYSTEM_PROMPT.count("Пример") >= 3

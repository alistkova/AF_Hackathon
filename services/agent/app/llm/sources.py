"""Маркеры источников Альфа-Банка и хелперы для их детекции."""

ALFA_SOURCE_PATTERNS = (
    "ALFA-",
    "ALFA_",
    "FI_Strategy",
    "Inflation_tracker",
    "Gold_digest",
    "Бонд_Бриф",
    "Commodities",
    "macro_stat",
    "consumption",
    "Russian_Steels",
    "Policy_Rate",
    "Macro_Insights",
)


def is_alfa_source(source_id: str) -> bool:
    return any(p in source_id for p in ALFA_SOURCE_PATTERNS)

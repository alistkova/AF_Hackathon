import pytest


def test_budget_accumulates(monkeypatch, tmp_path):
    monkeypatch.setenv("ALFA_AGENT_STATE_DIR", str(tmp_path))
    import importlib

    from app.llm import budget

    importlib.reload(budget)

    assert budget.today_usage()["calls"] == 0
    budget.add_usage(1000, 500)
    entry = budget.today_usage()
    assert entry["calls"] == 1
    assert entry["input_tokens"] == 1000
    assert entry["output_tokens"] == 500
    assert entry["cost_rub"] > 0


def test_budget_exceeded(monkeypatch, tmp_path):
    monkeypatch.setenv("ALFA_AGENT_STATE_DIR", str(tmp_path))
    import importlib

    from app.llm import budget

    importlib.reload(budget)
    budget.DAILY_BUDGET_RUB = 0.001
    budget.add_usage(10_000_000, 0)
    assert budget.is_exceeded()
    with pytest.raises(budget.BudgetExceeded):
        budget.check()

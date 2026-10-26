# Agent service

Stateless Python service that receives company context and evidence and returns a structured insight over SSE.

```bash
nix develop
cd services/agent
uv sync
uv run uvicorn app.main:app --reload --port 8090
```

Modes:

- `AGENT_MODE=stub` returns deterministic steps and an insight without an LLM.
- `AGENT_MODE=live` uses an OpenAI-compatible endpoint configured by `LLM_BASE_URL`, `LLM_API_KEY`, `LLM_MODEL`.
  Optional `LLM_FALLBACK_BASE_URL` is tried only when the primary endpoint is unreachable.

The contract is `contracts/openapi/agent.yaml`; SSE payloads are described in `contracts/events/agent-events.json`.

## Local models

Generation always goes through an OpenAI-compatible API (`LLM_BASE_URL`); there is no local generative model and no GPU requirement. The only local model is the RAG embedding encoder `sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2`, which runs on CPU and is only touched in `AGENT_MODE=live` or by the standalone RAG CLI. How to pre-download it and run offline (`HF_HOME`, `HF_HUB_OFFLINE`, `TRANSFORMERS_OFFLINE`) is described in the root [README](../../README.md#rag-по-документам).

## Quality tooling

- `make eval` runs the live LLM over `tests/fixtures/eval` and writes a report to `reports/`.
- `make eval-stub` runs the same fixtures through `StubAnalyzer` (no tokens) and checks the same thresholds.
- `make metrics` summarizes the JSONL product metrics in `METRICS_DIR` (`services/agent/metrics` by default; override with `METRICS_DIR`).
- Daily LLM budget is tracked in `ALFA_AGENT_STATE_DIR` (default `~/.alfa_agent`). The counter is per agent process, so horizontally scaled deployments must treat it as a soft limit.

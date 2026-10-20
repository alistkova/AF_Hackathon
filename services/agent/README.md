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

The contract is `contracts/openapi/agent.yaml`; SSE payloads are described in `contracts/events/agent-events.json`.

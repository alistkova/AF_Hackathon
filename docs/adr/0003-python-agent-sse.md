# ADR 0003: Python agent with SSE

## Status

Accepted.

## Context

The AI team owns the agent and wants the Python ecosystem. The UI must show multi-step agent progress without blocking on the LLM.

## Decision

The agent is a stateless Python/FastAPI service. Insight sends company context and evidence via `POST /v1/analyses` with `Accept: text/event-stream`. The agent emits step events and one structured result or error. Insight persists job state, relays public SSE through gateway and owns retries.

The agent has no database access. It returns source IDs from the evidence it received; insight resolves them to real URLs.

## Consequences

- Python dependencies and model choices do not affect the Go build.
- The agent can be replaced or scaled independently.
- Insight is the single source of truth for job state.
- SSE must be unbuffered through gateway and nginx.

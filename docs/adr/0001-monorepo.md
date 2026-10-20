# ADR 0001: Polyglot monorepo

## Status

Accepted.

## Context

A two-person backend/web team and a separate AI team need to deliver several services in a short hackathon. Cross-service changes and contract changes are frequent.

## Decision

Use one repository containing Go services, the Python agent, the web client, contracts and infrastructure.

## Consequences

- Atomic changes across gateway, insight, agent and web.
- One Compose stack and one CI pipeline.
- Service boundaries must be enforced by code review, database ownership and import rules rather than repository boundaries.
- The agent can be extracted later without changing callers because insight talks to it over HTTP/SSE.

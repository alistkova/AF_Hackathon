# ADR 0002: One PostgreSQL instance, one database per service

## Status

Accepted.

## Context

The design requires real service boundaries without operating four database servers during the hackathon.

## Decision

Run one PostgreSQL container. Create `company_db`, `content_db`, `insight_db` and `identity_db` with separate roles. Each service connects only to its own database.

## Consequences

- Real ownership boundaries and independent migrations.
- No cross-database joins or queries.
- One backup and one local container.
- Production can split databases onto separate instances without changing service code.

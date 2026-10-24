# Альфа Хак 2026 — новостная аналитика

Персональная лента событий для крупного бизнеса: что случилось, почему это важно для компании, риски и возможности, позиция банка, вывод AI и источники.

## Architecture

```text
Browser
   |  HTTP + SSE
   v
Gateway (Go/Gin)
   |-- Company (Go)  -> PostgreSQL company_db
   |-- Content (Go)  -> PostgreSQL content_db + MinIO
   |-- Insight (Go)  -> PostgreSQL insight_db
   |                      |
   |                      +-- Agent (Python/FastAPI, SSE)
   +-- Web (React + TypeScript + Tailwind)
```

One PostgreSQL instance with separate databases and roles per service. Redis is provisioned for later queues; MinIO is an optional Compose profile for raw documents. The agent is stateless: insight owns retrieval, persistence, retries and the browser SSE stream.

## Quick start

The code lives in this repository folder, next to the read-only `design/` materials. Run all commands from the repository root.

### Laptop demo, Docker only

```bash
cp .env.example .env
make demo
```

Open <http://localhost:3000>. The demo starts Postgres, Redis, all Go services, the Python agent and the web app, then seeds companies and news. MinIO is an optional Compose profile because its public registry requires separate access; the current content scaffold does not use object storage yet.

Reset an empty demo:

```bash
make demo-reset
```

If host port 8080 is already in use, set `GATEWAY_PORT=18080` in `.env`; `make demo`, `make smoke` and the printed URL use that value. `WEB_PORT` works the same way.

### Without Nix

Install Go 1.26+, Node 24+, pnpm 11+, Python 3.13+, uv, golangci-lint and ruff. Docker is required for infrastructure and the full demo.

```bash
cp .env.example .env
make bootstrap
make infra
make dev
```

### Nix development shell

```bash
nix develop
cp .env.example .env
make bootstrap
make infra
make dev
```

`make dev` starts infrastructure in Docker and services on the host:

- web: <http://localhost:5173>
- gateway: <http://localhost:8080>
- company: <http://localhost:8081>
- content: <http://localhost:8082>
- insight: <http://localhost:8083>
- agent: <http://localhost:8090>

The dev shell provides Go, Node, pnpm, Python, uv, ruff, PostgreSQL client, Redis client, MinIO client and Docker is used for infrastructure.

### Agent development

```bash
nix develop
cd services/agent
uv sync
uv run uvicorn app.main:app --reload --port 8090
```

`AGENT_MODE=stub` works without an LLM. Set `AGENT_MODE=live`, `LLM_BASE_URL`, `LLM_API_KEY` and `LLM_MODEL` for an OpenAI-compatible endpoint.

## Commands

```bash
make infra       # postgres, redis
make infra-minio # optional MinIO profile
make migrate     # run service migrations
make seed        # import companies and news fixtures
make dev         # native services + Docker infrastructure
make demo        # full Compose stack
make test        # Go, web and agent tests
make lint        # gofmt, golangci-lint, ESLint, TypeScript, ruff
make smoke       # end-to-end SSE smoke test
make gen         # generate contract types
```

## Service boundaries

- Web talks only to gateway.
- Every service owns its database and role. No cross-database queries.
- Services do not import each other's `internal` packages.
- The agent only reasons over the evidence it receives and returns source IDs from that evidence.
- Public SSE goes through gateway -> insight. Agent SSE is internal.

## Company API

The company service owns company profiles in `company_db`. Clients call it through the gateway:

```bash
curl -s localhost:8080/api/v1/companies
curl -s localhost:8080/api/v1/companies/magnit
curl -s localhost:8080/api/v1/companies/magnit/relations
```

`GET /api/v1/companies` returns `{"items":[...]}` ordered by name. `GET /api/v1/companies/{companyId}` returns one company with `people` and `relations`, or a `404` error. Relations resolve company groups, shared people and payment counterparties from `company_db`.

Demo data is loaded from `seed/companies.csv` by `make seed`. The hackathon datasets are imported with the same command:

```bash
go run ./services/company/cmd/seed \
  -file path/to/df_Uid_data_hack.csv \
  -people path/to/df_Fid_Uid_link_hack.csv \
  -payments path/to/payments.csv
```

Service details, the data model, CSV format and local debugging steps are in [services/company/README.md](services/company/README.md). The service contract is [contracts/openapi/company.yaml](contracts/openapi/company.yaml).

## Content API

`GET /api/v1/news` returns the AI team news format: `news_id`, `title`, `published_at`, `source_name`, `source_url`, `full_text`, `tags`, `target_sectors`. The schema is in `contracts/openapi/content.yaml`.

## SSE contract

Start a job:

```bash
curl -s -X POST localhost:8080/api/v1/insights/jobs \
  -H 'Content-Type: application/json' \
  -d '{"company_id":"magnit","event":{"id":"news-001","title":"Магнит расширяет логистику"}}'
```

Stream it:

```bash
curl -N localhost:8080/api/v1/insights/jobs/<job_id>/events
```

The stream emits `status`, `step`, `result` and `error` events. `result` and `error` are terminal. Reconnect with `Last-Event-ID` to replay events after the last received sequence number.

Contracts live in `contracts/`; SSE framing is documented in `contracts/README.md`.

## Repository layout

```text
contracts/          OpenAPI + SSE event schemas
services/gateway/   public BFF, proxying, SSE relay
services/company/   companies, people, relations
services/content/   documents and chunks
services/insight/   feed, jobs, SSE, agent orchestration
services/agent/     Python/FastAPI agent
web/                React + TypeScript + Tailwind
infra/              Postgres roles/databases, MinIO
seed/               demo companies and news
scripts/            dev, demo, migrate, seed, smoke, gen
```

## Current scaffold status

- M0 vertical slice: gateway, company, web, Compose, seed, smoke.
- SSE job flow works end to end with the Python stub agent.
- Content document storage and ingestion worker are scaffolds.
- Insight jobs are in-memory for now; `analysis_jobs`/`analysis_events` tables are reserved for durable persistence.
- Identity is deferred; `identity_db` and role exist for later extraction.

## Contributing

Commit message conventions are documented in [CONTRIBUTING.md](CONTRIBUTING.md).

## Module path

The Go module currently uses `github.com/alfa-hack/alfa-hack`. Change `module` in `go.mod` and imports if your organization uses a different path.

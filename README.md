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
The agent includes a RAG system for Russian financial news PDFs with incremental updates.

## Quick start

The code lives in this repository folder, next to the read-only `design/` materials. Run all commands from the repository root.

### Laptop demo, Docker only

```bash
cp .env.example .env
make demo
```

Open <http://localhost:3000>. The demo starts Postgres, Redis, all Go services, the Python agent and the web app, then seeds companies, people, payments and news. MinIO is an optional Compose profile because its public registry requires separate access; the current content scaffold does not use object storage yet.

`make demo` also creates two demo accounts and prints them:

| Account | Role | Sees |
| --- | --- | --- |
| `demo-admin@alfa.local` | `admin` | every company, can grant access |
| `demo-client@alfa.local` | `analyst` | only the companies granted to it |

The password for both is `demo-password` (`DEMO_PASSWORD`). Override the emails with `DEMO_ADMIN_EMAIL` and `DEMO_CLIENT_EMAIL`, and the granted companies with `DEMO_CLIENT_COMPANIES`. `make demo-users` recreates the accounts on an already running stack.

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

### RAG system development

The agent includes a RAG system for Russian financial news (ALFA_Russian_Steels format) at `services/agent/app/rag_system/`.

The agent computes embeddings locally and stores chunks through the content service (`CONTENT_URL`, default `http://localhost:8082`). The content service owns the `rag_chunks` table in `content_db` and applies `company_id`, `sector` and `published_at` filters before the similarity search, so the agent never connects to `content_db`. Start the stack or run `make migrate` to create the table.

OCR for scanned PDFs is optional: install the `tesseract` binary with Russian language data, run `cd services/agent && uv sync --extra ocr` (or `make rag-install`) and set `RAG_USE_OCR=True`.

```bash
# Install RAG dependencies (embeddings need the hf extra)
make rag-install

# Build index from PDFs in services/agent/data/
make rag-build

# Query the RAG system
make rag-query Q="ваш вопрос"

# Query with metadata filters
make rag-search Q="выручка" COMPANY_ID=magnit AFTER=2026-01-01

# Incremental update for new PDFs
make rag-update

# Show index statistics
make rag-stats
```

Requires Ollama with models (qwen2.5:32b, DeepSeek, etc.). Set `OLLAMA_BASE_URL` and `OLLAMA_MODEL` in `.env`.

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

# RAG system
make rag-install  # install RAG dependencies
make rag-build    # build index from PDFs
make rag-update   # incremental update
make rag-query Q="question"  # query RAG
make rag-search Q="query" COMPANY_ID=magnit  # search chunks without the LLM
make rag-list COMPANY_ID=magnit  # list chunks by metadata
make rag-stats    # index statistics
make rag-clean    # remove the local processed-files cache (vectors stay in the content service)
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

## Identity API

The identity service owns users, sessions and company access in `identity_db`. The gateway forwards `/api/v1/auth`, `/api/v1/me` and `/api/v1/access` to it:

```bash
curl -s -X POST localhost:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"owner@example.com","password":"secret-pass"}'

curl -s -c /tmp/alfa.jar -X POST localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"owner@example.com","password":"secret-pass"}'

curl -s -b /tmp/alfa.jar localhost:8080/api/v1/me
curl -s -b /tmp/alfa.jar localhost:8080/api/v1/me/companies
```

Passwords are bcrypt hashes and only SHA-256 token hashes are persisted. Sessions live in the `alfa_session` cookie (`SESSION_COOKIE_NAME`) or in an `Authorization: Bearer` header. The first registered user becomes `admin`, later users become `analyst`; granting and revoking company access requires `admin`.

`/api/v1/companies` requires the same session. Without it the gateway returns `401`; an `analyst` only gets the companies granted to it, an `admin` gets everything. See [Access model](#access-model).

Service details and configuration are in [services/identity/README.md](services/identity/README.md). The service contract is [contracts/openapi/identity.yaml](contracts/openapi/identity.yaml).

## Content API

`GET /api/v1/news` returns the AI team news format (`document_id`, `news_id`, `title`, `published_at`, `source_name`, `source_url`, `full_text`, `tags`, `target_sectors`) and accepts `kind`, `sector`, `tag`, `source`, `from`, `to`, `limit` and `offset`. The total is returned in `X-Total-Count`.

Ingestion goes through `POST /api/v1/documents` (JSON) or `POST /api/v1/documents/pdf` (multipart). Both normalise the text, reject duplicates by canonical URL and by a content hash, and split the body into overlapping chunks. Ranked search across news and PDFs is `GET /api/v1/search?q=`.

```bash
curl -s -X POST localhost:8080/api/v1/documents -H 'Content-Type: application/json' \
  -d '{"items":[{"news_id":"n1","title":"Ставка","full_text":"..."}]}'

curl -s -X POST localhost:8080/api/v1/documents/pdf -F 'file=@material.pdf' -F 'source_name=Альфа-Банк'

curl -s 'localhost:8080/api/v1/search?q=ключевая%20ставка'
```

PDF text extraction uses `pdftotext` from poppler-utils, which the content image installs. Raw files land in the `contentdata` volume. The schema is in `contracts/openapi/content.yaml`, the details are in [services/content/README.md](services/content/README.md).

RAG chunks are stored by the content service as well: `POST /api/v1/rag/chunks` upserts embedded chunks, `POST /api/v1/rag/chunks/search` and `POST /api/v1/rag/chunks/list` query them by metadata, `GET /api/v1/rag/chunks/count` counts them and `DELETE /api/v1/rag/chunks` clears the table. The agent uses these endpoints and never opens `content_db`.

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
services/identity/  users, sessions, company access
services/insight/   feed, jobs, SSE, agent orchestration
services/agent/     Python/FastAPI agent
services/agent/app/rag_system/   RAG system for Russian financial news
web/                React + TypeScript + Tailwind
infra/              Postgres roles/databases, MinIO
seed/               demo companies, people, payments and news
scripts/            dev, demo, migrate, seed, bootstrap-users, smoke, gen
```

## Current scaffold status

- M0 vertical slice: gateway, company, web, Compose, seed, smoke.
- M1 complete: identity with sessions, per-user company access enforced in the gateway, CSV import of companies, people and payments, company profiles and relations.
- SSE job flow works end to end with the Python stub agent.
- Content owns news and PDF documents: ingestion with deduplication, chunking, full-text search and raw file storage.
- The ingestion worker is still a no-op; documents arrive through the API and the seed command.
- Insight jobs are in-memory for now; `analysis_jobs`/`analysis_events` tables are reserved for durable persistence.
- Identity is implemented: register, login, logout, `/me`, and per-user company access with admin-only grants.

## Access model

The gateway validates the session against identity on every `/api/v1/companies` request and never trusts the client:

- `admin` sees every company.
- `analyst` sees only the companies in `company_access`.
- The list is filtered by forwarding the allowed ids to the company service, and a detail request for a company outside that set returns `404`.
- Company, content and insight are also published on their own ports for local debugging. Only the gateway applies access control, so public traffic must go through it.

## Contributing

Commit message conventions are documented in [CONTRIBUTING.md](CONTRIBUTING.md).

## Module path

The Go module currently uses `github.com/alfa-hack/alfa-hack`. Change `module` in `go.mod` and imports if your organization uses a different path.

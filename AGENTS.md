# Agent instructions

## Toolchain

Nix is optional; not everyone on the team uses NixOS.

On NixOS:

```bash
nix develop
make bootstrap
make infra
```

On other systems, install Go 1.26+, Node 24+, pnpm 11+, Python 3.13+, uv, golangci-lint, ruff and Docker, then run the same Make targets. `make demo` requires only Docker. Do not make Nix a requirement for building or running the project.

## Verification

Run before handing off:

```bash
make lint
make test
make smoke
```

`make lint` runs gofmt, go vet, golangci-lint, ESLint, TypeScript, Ruff format/check. `make test` runs Go race tests, Vitest and Pytest.

`make smoke` requires the stack to be running. For a Docker-only check use `make demo` in one terminal and `make smoke` in another.

## Rules

- Keep the monorepo structure. Do not add separate repositories or git submodules.
- Web calls gateway only. Do not call company, content, insight or agent directly from the browser.
- Each service owns its database and role. Do not query another service's database.
- Do not import another service's `internal` packages.
- Keep the agent stateless. Insight owns jobs, persistence, retries and public SSE.
- Keep `contracts/` in sync with handlers and clients. Public SSE payloads must validate against `contracts/events/insight-progress.json`.
- Do not commit `.env`, API keys or LLM credentials.
- Do not add comments to code unless a compiler directive requires it.
- Prefer standard library and the existing stack. Do not add a new framework without an ADR.

## Commit messages

Use `<type>: <description>`, for example `feat: add company feed`.

Allowed types: `feat`, `fix`, `docs`, `test`, `refactor`, `chore`, `ci`. Do not use scopes such as `feat(web): ...`. See `CONTRIBUTING.md` for details.

## Service map

- `services/gateway`: public API, request IDs, auth guard, reverse proxy, SSE relay.
- `services/identity`: users, sessions and company access, `identity_db`.
- `services/company`: companies and relations, `company_db`.
- `services/content`: documents and chunks, `content_db`, MinIO.
- `services/insight`: feed, jobs, SSE, agent orchestration, `insight_db`.
- `services/agent`: Python/FastAPI SSE agent with `stub` and `live` modes.
- `web`: React/TypeScript/Tailwind client.

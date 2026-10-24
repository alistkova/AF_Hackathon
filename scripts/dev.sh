#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

if [[ ! -f .env ]]; then
  cp .env.example .env
fi
set -a
source .env
set +a

if [[ "${SKIP_INFRA:-0}" != "1" ]]; then
  docker compose -f compose.dev.yaml up -d --wait
fi

pids=()
cleanup() {
  if [[ ${#pids[@]} -gt 0 ]]; then
    kill "${pids[@]}" 2>/dev/null || true
    wait "${pids[@]}" 2>/dev/null || true
  fi
}
trap cleanup EXIT INT TERM

go run ./services/company/cmd/api &
pids+=("$!")
go run ./services/content/cmd/api &
pids+=("$!")
go run ./services/insight/cmd/api &
pids+=("$!")
go run ./services/identity/cmd/api &
pids+=("$!")
go run ./services/gateway/cmd/api &
pids+=("$!")

if command -v uv >/dev/null 2>&1 && [[ "${SKIP_AGENT:-0}" != "1" ]]; then
  (cd services/agent && uv run uvicorn app.main:app --host 0.0.0.0 --port 8090 --reload) &
  pids+=("$!")
else
  export ANALYZER_MODE=stub
fi

pnpm --dir web run dev &
pids+=("$!")

for _ in $(seq 1 40); do
  if curl -fsS "http://localhost:8081/healthz" >/dev/null 2>&1 && curl -fsS "http://localhost:8082/healthz" >/dev/null 2>&1; then
    bash scripts/seed.sh
    break
  fi
  sleep 1
done

wait

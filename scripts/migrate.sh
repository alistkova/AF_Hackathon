#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

if [[ -f .env ]]; then
  set -a
  source .env
  set +a
fi

MIGRATE_ONLY=1 go run ./services/company/cmd/api
MIGRATE_ONLY=1 go run ./services/content/cmd/api
MIGRATE_ONLY=1 go run ./services/insight/cmd/api

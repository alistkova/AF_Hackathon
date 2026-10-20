#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

if [[ -f .env ]]; then
  set -a
  source .env
  set +a
fi

go run ./services/company/cmd/seed -file seed/companies.csv
go run ./services/content/cmd/seed -file seed/news/sample.json

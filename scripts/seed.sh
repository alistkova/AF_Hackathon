#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

if [[ -f .env ]]; then
  set -a
  source .env
  set +a
fi

company_args=(-file seed/companies.csv)
if [[ -f seed/people.csv ]]; then
  company_args+=(-people seed/people.csv)
fi
if [[ -f seed/payments.csv ]]; then
  company_args+=(-payments seed/payments.csv)
fi

go run ./services/company/cmd/seed "${company_args[@]}"
go run ./services/content/cmd/seed -file seed/news/sample.json

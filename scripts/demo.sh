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

docker compose up -d --build --wait
docker compose exec -T company /app/seed -file /seed/companies.csv -people /seed/people.csv -payments /seed/payments.csv
docker compose exec -T content /app/content-seed -file /seed/news/sample.json
bash scripts/bootstrap-users.sh

gateway_port="${GATEWAY_PORT:-8080}"
web_port="${WEB_PORT:-3000}"

echo
echo "Demo is running:"
echo "  web:     http://localhost:${web_port}"
echo "  gateway: http://localhost:${gateway_port}/healthz"

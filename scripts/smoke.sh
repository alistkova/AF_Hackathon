#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
if [[ -f .env ]]; then
  set -a
  source .env
  set +a
fi
BASE_URL="${BASE_URL:-http://localhost:${GATEWAY_PORT:-8080}}"
TMP_FILE="$(mktemp)"
trap 'rm -f "$TMP_FILE"' EXIT

curl -fsS "$BASE_URL/healthz" >/dev/null

companies="$(curl -fsS "$BASE_URL/api/v1/companies")"
company_id="$(python3 -c 'import json,sys; items=json.load(sys.stdin)["items"]; assert items; print(items[0]["id"])' <<<"$companies")"

payload="$(python3 -c 'import json,sys; print(json.dumps({"company_id": sys.argv[1], "event": {"id": "smoke-event", "title": "Проверка SSE", "summary": "Сквозной тест gateway, insight и агента", "source": "smoke", "url": "https://example.com/smoke"}}, ensure_ascii=False))' "$company_id")"

job="$(curl -fsS -X POST "$BASE_URL/api/v1/insights/jobs" \
  -H 'Content-Type: application/json' \
  -d "$payload")"
job_id="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["job_id"])' <<<"$job")"
test "$job_id" != "null"

timeout 60 curl -fsSN "$BASE_URL/api/v1/insights/jobs/$job_id/events" -o "$TMP_FILE"
grep -q 'event: step' "$TMP_FILE"
grep -q 'event: result' "$TMP_FILE"

echo "smoke ok: job $job_id"

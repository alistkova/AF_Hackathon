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
COOKIE_JAR="$(mktemp)"
trap 'rm -f "$TMP_FILE" "$COOKIE_JAR"' EXIT

curl -fsS "$BASE_URL/healthz" >/dev/null

email="smoke-$(date +%s)-$$@example.com"
password="smoke-password"

curl -fsS -X POST "$BASE_URL/api/v1/auth/register" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"$email\",\"password\":\"$password\"}" >/dev/null

curl -fsS -c "$COOKIE_JAR" -X POST "$BASE_URL/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"$email\",\"password\":\"$password\"}" >/dev/null

curl -fsS -b "$COOKIE_JAR" "$BASE_URL/api/v1/me" \
  | python3 -c 'import json,sys; me=json.load(sys.stdin); assert me["email"] == sys.argv[1] and me["role"], me' "$email"

companies="$(curl -fsS "$BASE_URL/api/v1/companies")"
company_id="$(python3 -c 'import json,sys; items=json.load(sys.stdin)["items"]; assert items; print(items[0]["id"])' <<<"$companies")"

profile="$(curl -fsS "$BASE_URL/api/v1/companies/$company_id")"
python3 -c 'import json,sys; item=json.load(sys.stdin); assert item["id"] == sys.argv[1] and item["name"]' "$company_id" <<<"$profile"

payload="$(python3 -c 'import json,sys; print(json.dumps({"company_id": sys.argv[1], "event": {"id": "smoke-event", "title": "Проверка SSE", "summary": "Сквозной тест gateway, insight и агента", "source": "smoke", "url": "https://example.com/smoke"}}, ensure_ascii=False))' "$company_id")"

job="$(curl -fsS -X POST "$BASE_URL/api/v1/insights/jobs" \
  -H 'Content-Type: application/json' \
  -d "$payload")"
job_id="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["job_id"])' <<<"$job")"
test "$job_id" != "null"

timeout 60 curl -fsSN "$BASE_URL/api/v1/insights/jobs/$job_id/events" -o "$TMP_FILE"
grep -q 'event: step' "$TMP_FILE"
grep -q 'event: result' "$TMP_FILE"

echo "smoke ok: identity $email, job $job_id"

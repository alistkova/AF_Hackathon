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
ADMIN_EMAIL="${DEMO_ADMIN_EMAIL:-demo-admin@alfa.local}"
CLIENT_EMAIL="${DEMO_CLIENT_EMAIL:-demo-client@alfa.local}"
PASSWORD="${DEMO_PASSWORD:-demo-password}"

TMP_FILE="$(mktemp)"
ADMIN_JAR="$(mktemp)"
CLIENT_JAR="$(mktemp)"
trap 'rm -f "$TMP_FILE" "$ADMIN_JAR" "$CLIENT_JAR"' EXIT

login() {
  local email="$1" jar="$2"
  curl -fsS -c "$jar" -o /dev/null -X POST "$BASE_URL/api/v1/auth/login" \
    -H 'Content-Type: application/json' \
    -d "{\"email\":\"$email\",\"password\":\"$PASSWORD\"}"
}

json_items() {
  python3 -c 'import json,sys; print(" ".join(item["id"] for item in json.load(sys.stdin)["items"]))' | tr -d '\r'
}

curl -fsS "$BASE_URL/healthz" >/dev/null

if ! login "$ADMIN_EMAIL" "$ADMIN_JAR"; then
  echo "smoke: cannot sign in as $ADMIN_EMAIL; run 'make demo' or 'make dev' first" >&2
  exit 1
fi
login "$CLIENT_EMAIL" "$CLIENT_JAR"

status="$(curl -sS -o /dev/null -w '%{http_code}' "$BASE_URL/api/v1/companies")"
if [[ "$status" != "401" ]]; then
  echo "smoke: unauthenticated company list returned $status, want 401" >&2
  exit 1
fi

admin_role="$(curl -fsS -b "$ADMIN_JAR" "$BASE_URL/api/v1/me" \
  | python3 -c 'import json,sys; print(json.load(sys.stdin)["role"])' | tr -d '\r')"
if [[ "$admin_role" != "admin" ]]; then
  echo "smoke: $ADMIN_EMAIL has role $admin_role, want admin" >&2
  exit 1
fi

admin_companies="$(curl -fsS -b "$ADMIN_JAR" "$BASE_URL/api/v1/companies" | json_items)"
client_companies="$(curl -fsS -b "$CLIENT_JAR" "$BASE_URL/api/v1/companies" | json_items)"

if [[ -z "$admin_companies" ]]; then
  echo "smoke: admin sees no companies; run 'make demo' or 'make dev' first" >&2
  exit 1
fi
if [[ -z "$client_companies" ]]; then
  echo "smoke: client sees no companies; run 'make demo' or 'make dev' first" >&2
  exit 1
fi
client_primary="$(curl -fsS -b "$CLIENT_JAR" "$BASE_URL/api/v1/me" \
  | python3 -c 'import json,sys; print(json.load(sys.stdin).get("primary_company_id", ""))' | tr -d '\r')"
if [[ -z "$client_primary" ]]; then
  echo "smoke: client has no primary company; run 'make demo' or 'make dev' first" >&2
  exit 1
fi

memberships="$(curl -fsS -b "$CLIENT_JAR" "$BASE_URL/api/v1/me/memberships")"
python3 -c '
import json,sys
payload = json.load(sys.stdin)
items = payload["items"]
assert items, payload
for item in items:
    assert item["role"] in ("owner", "member"), item
' <<<"$memberships"

if [[ "$client_companies" == "$admin_companies" ]]; then
  echo "smoke: admin and client have identical memberships; skipping the forbidden-company checks" >&2
fi

forbidden=""
for company in $admin_companies; do
  if [[ " $client_companies " != *" $company "* ]]; then
    forbidden="$company"
    break
  fi
done
if [[ -n "$forbidden" ]]; then
  status="$(curl -sS -o /dev/null -w '%{http_code}' -b "$CLIENT_JAR" "$BASE_URL/api/v1/companies/$forbidden")"
  if [[ "$status" != "404" ]]; then
    echo "smoke: client opened forbidden company $forbidden with $status, want 404" >&2
    exit 1
  fi
fi

company_id="${client_companies%% *}"

status="$(curl -sS -o /dev/null -w '%{http_code}' "$BASE_URL/api/v1/feed?company_id=$company_id")"
if [[ "$status" != "401" ]]; then
  echo "smoke: unauthenticated feed returned $status, want 401" >&2
  exit 1
fi

feed="$(curl -fsS -b "$CLIENT_JAR" "$BASE_URL/api/v1/feed?company_id=$company_id&sort=date&limit=1")"
python3 -c 'import json,sys
payload = json.load(sys.stdin)
items = payload["items"]
assert isinstance(payload.get("total"), int), payload
assert items, "feed is empty; seed content news first"
for item in items:
    assert isinstance(item.get("relevance"), (int, float)), item
    assert isinstance(item.get("reasons"), list), item
    assert isinstance(item.get("tags"), list), item
    assert isinstance(item.get("target_sectors"), list), item
    assert isinstance(item.get("source"), str), item
    assert isinstance(item.get("url"), str), item
facets = payload.get("facets")
assert isinstance(facets, dict), payload
for key in ("sources", "tags", "sectors"):
    assert isinstance(facets.get(key), list), payload
' <<<"$feed"

if [[ -n "$forbidden" ]]; then
  status="$(curl -sS -o /dev/null -w '%{http_code}' -b "$CLIENT_JAR" "$BASE_URL/api/v1/feed?company_id=$forbidden")"
  if [[ "$status" != "404" ]]; then
    echo "smoke: client read the forbidden feed $forbidden with $status, want 404" >&2
    exit 1
  fi

  forbidden_payload="$(python3 -c 'import json,sys; print(json.dumps({"company_id": sys.argv[1], "event": {"id": "smoke-forbidden", "title": "Forbidden"}}))' "$forbidden")"
  status="$(curl -sS -o /dev/null -w '%{http_code}' -b "$CLIENT_JAR" -X POST "$BASE_URL/api/v1/insights/jobs" \
    -H 'Content-Type: application/json' -d "$forbidden_payload")"
  if [[ "$status" != "404" ]]; then
    echo "smoke: client created a job for $forbidden with $status, want 404" >&2
    exit 1
  fi
fi

news="$(curl -fsS "$BASE_URL/api/v1/news?limit=5")"
python3 -c 'import json,sys; assert isinstance(json.load(sys.stdin), list)' <<<"$news"

search="$(curl -fsS --get --data-urlencode "q=ставка" "$BASE_URL/api/v1/search")"
python3 -c 'import json,sys; assert "items" in json.load(sys.stdin)' <<<"$search"

profile="$(curl -fsS -b "$CLIENT_JAR" "$BASE_URL/api/v1/companies/$company_id")"
python3 -c 'import json,sys; item=json.load(sys.stdin); assert item["id"] == sys.argv[1] and item["name"], item' "$company_id" <<<"$profile"

relations="$(curl -fsS -b "$CLIENT_JAR" "$BASE_URL/api/v1/companies/$company_id/relations")"
python3 -c 'import json,sys; items=json.load(sys.stdin)["items"]; [print(item["company_id"], ",".join(item["types"])) for item in items]' <<<"$relations" >/dev/null

payload="$(python3 -c 'import json,sys; print(json.dumps({"company_id": sys.argv[1], "event": {"id": "smoke-event", "title": "Проверка SSE", "summary": "Сквозной тест gateway, insight и агента", "source": "smoke", "url": "https://example.com/smoke"}}, ensure_ascii=False))' "$company_id")"

job="$(curl -fsS -b "$ADMIN_JAR" -X POST "$BASE_URL/api/v1/insights/jobs" \
  -H 'Content-Type: application/json' \
  -d "$payload")"
job_id="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["job_id"])' <<<"$job")"
test "$job_id" != "null"

status="$(curl -sS -o /dev/null -w '%{http_code}' "$BASE_URL/api/v1/insights/jobs/$job_id")"
if [[ "$status" != "401" ]]; then
  echo "smoke: unauthenticated job read returned $status, want 401" >&2
  exit 1
fi

timeout 60 curl -fsSN -b "$ADMIN_JAR" "$BASE_URL/api/v1/insights/jobs/$job_id/events" -o "$TMP_FILE"
grep -q 'event: step' "$TMP_FILE"
grep -q 'event: result' "$TMP_FILE"

echo "smoke ok: client sees [$client_companies], job $job_id"

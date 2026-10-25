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
COMPANY_URL="${COMPANY_URL:-http://localhost:${COMPANY_PORT:-8081}}"
ADMIN_EMAIL="${DEMO_ADMIN_EMAIL:-demo-admin@alfa.local}"
CLIENT_EMAIL="${DEMO_CLIENT_EMAIL:-demo-client@alfa.local}"
PASSWORD="${DEMO_PASSWORD:-demo-password}"
ADMIN_COMPANIES="${DEMO_ADMIN_COMPANIES:-magnit severstal sber}"
CLIENT_COMPANIES="${DEMO_CLIENT_COMPANIES:-magnit severstal}"

register() {
  local email="$1"
  local code
  code="$(curl -sS -o /dev/null -w '%{http_code}' -X POST "$BASE_URL/api/v1/auth/register" \
    -H 'Content-Type: application/json' \
    -d "{\"email\":\"$email\",\"password\":\"$PASSWORD\"}")"
  case "$code" in
    201|409) ;;
    403)
      echo "bootstrap: registration is disabled; set IDENTITY_REGISTRATION_ENABLED=true to create demo users" >&2
      return 1
      ;;
    *)
      echo "bootstrap: register $email returned $code" >&2
      return 1
      ;;
  esac
}

admin_jar="$(mktemp)"
client_jar="$(mktemp)"
trap 'rm -f "$admin_jar" "$client_jar"' EXIT

register "$ADMIN_EMAIL"
register "$CLIENT_EMAIL"

curl -fsS -c "$admin_jar" -o /dev/null -X POST "$BASE_URL/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$PASSWORD\"}"

curl -fsS -c "$client_jar" -o /dev/null -X POST "$BASE_URL/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"$CLIENT_EMAIL\",\"password\":\"$PASSWORD\"}"

read -r role client_id < <(curl -fsS -b "$client_jar" "$BASE_URL/api/v1/me" \
  | python3 -c 'import json,sys; me=json.load(sys.stdin); print(me["role"], me["id"])' | tr -d '\r')
admin_id="$(curl -fsS -b "$admin_jar" "$BASE_URL/api/v1/me" \
  | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])' | tr -d '\r')"

admin_role="$(curl -fsS -b "$admin_jar" "$BASE_URL/api/v1/me" \
  | python3 -c 'import json,sys; print(json.load(sys.stdin)["role"])' | tr -d '\r')"

if [[ "$admin_role" != "admin" ]]; then
  echo "bootstrap: $ADMIN_EMAIL is '$admin_role', so company access cannot be granted." >&2
  echo "bootstrap: recreate the database with 'make demo-reset' to make it the first admin." >&2
  exit 0
fi

grant() {
  local user_id="$1" company="$2" role="$3"
  curl -sS -b "$admin_jar" -o /dev/null -w '%{http_code}' -X POST "$BASE_URL/api/v1/access/grant" \
    -H 'Content-Type: application/json' \
    -d "{\"user_id\":\"$user_id\",\"company_id\":\"$company\",\"role\":\"$role\"}" || true
}

company_ids() {
  python3 -c 'import json,sys; print(" ".join(item["company_id"] for item in json.load(sys.stdin)["items"]))' | tr -d '\r'
}

catalog_ids() {
  python3 -c 'import json,sys; print(" ".join(item["id"] for item in json.load(sys.stdin)["items"]))' | tr -d '\r'
}

append() {
  local list="$1" value="$2"
  if [[ -z "$list" ]]; then
    printf '%s\n' "$value"
  else
    printf '%s %s\n' "$list" "$value"
  fi
}

has_company() {
  local list="$1" company="$2"
  [[ " $list " == *" $company "* ]]
}

client_memberships="$(curl -fsS -b "$client_jar" "$BASE_URL/api/v1/me/memberships" | company_ids)"
admin_memberships="$(curl -fsS -b "$admin_jar" "$BASE_URL/api/v1/me/memberships" | company_ids)"
client_primary="$(curl -fsS -b "$client_jar" "$BASE_URL/api/v1/me" \
  | python3 -c 'import json,sys; print(json.load(sys.stdin).get("primary_company_id", ""))' | tr -d '\r')"
admin_primary="$(curl -fsS -b "$admin_jar" "$BASE_URL/api/v1/me" \
  | python3 -c 'import json,sys; print(json.load(sys.stdin).get("primary_company_id", ""))' | tr -d '\r')"

available=""
if ! available="$(curl -fsS "$COMPANY_URL/api/v1/companies" | catalog_ids)"; then
  available="$(curl -fsS -b "$admin_jar" "$BASE_URL/api/v1/companies" | catalog_ids)"
  echo "bootstrap: company service unavailable at $COMPANY_URL, using the admin's companies" >&2
fi

for company in $ADMIN_COMPANIES; do
  if has_company "$admin_memberships" "$company"; then
    continue
  fi
  code="$(grant "$admin_id" "$company" "member")"
  if [[ "$code" != "204" ]]; then
    echo "bootstrap: grant admin $company returned $code" >&2
    exit 1
  fi
  admin_memberships="$(append "$admin_memberships" "$company")"
done

for company in $CLIENT_COMPANIES; do
  if has_company "$client_memberships" "$company"; then
    continue
  fi
  if ! has_company "$available" "$company"; then
    echo "bootstrap: company $company not found, skipping" >&2
    continue
  fi
  code="$(grant "$client_id" "$company" "owner")"
  if [[ "$code" != "204" ]]; then
    echo "bootstrap: grant $company returned $code" >&2
    exit 1
  fi
  client_memberships="$(append "$client_memberships" "$company")"
done

if [[ -z "${client_memberships// /}" && -n "$available" ]]; then
  first="${available%% *}"
  code="$(grant "$client_id" "$first" "owner")"
  if [[ "$code" != "204" ]]; then
    echo "bootstrap: grant $first returned $code" >&2
    exit 1
  fi
  client_memberships="$first"
fi

if [[ -z "$client_memberships" ]]; then
  echo "bootstrap: no company is available for $CLIENT_EMAIL; seed companies first" >&2
else
  CLIENT_COMPANIES="$client_memberships"
fi

if [[ -z "$client_primary" && -n "$client_memberships" ]]; then
  primary_company="${client_memberships%% *}"
  curl -fsS -b "$client_jar" -o /dev/null -X PATCH "$BASE_URL/api/v1/me" \
    -H 'Content-Type: application/json' \
    -d "{\"primary_company_id\":\"$primary_company\"}"
fi

if [[ -z "$admin_primary" && -n "$admin_memberships" ]]; then
  admin_primary="${admin_memberships%% *}"
  curl -fsS -b "$admin_jar" -o /dev/null -X PATCH "$BASE_URL/api/v1/me" \
    -H 'Content-Type: application/json' \
    -d "{\"primary_company_id\":\"$admin_primary\"}"
fi

echo "bootstrap: admin $ADMIN_EMAIL ($admin_role), companies: $admin_memberships"
echo "bootstrap: client $CLIENT_EMAIL ($role), companies: $client_memberships"
echo "bootstrap: password for both accounts: $PASSWORD"

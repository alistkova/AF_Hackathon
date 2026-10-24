# Identity service

Go/Gin service for users, sessions and company access.

Identity owns `identity_db` and is the only service that stores credentials. Other services
never read its tables; the gateway forwards `/api/v1/auth`, `/api/v1/me` and `/api/v1/access`
requests to it.

## Behaviour

- Passwords are hashed with bcrypt (cost 10). Plain-text rows left by older builds are
  upgraded to bcrypt on the next successful login.
- Session tokens are random 256-bit values. Only the SHA-256 hash of a token is stored, so a
  database dump does not leak usable sessions.
- Sessions expire after `IDENTITY_SESSION_TTL` (default `168h`). Expired rows are pruned on
  every login and rejected on every read.
- The client may authenticate with the session cookie or with `Authorization: Bearer <token>`.
- The first registered user becomes `admin`; every later registration becomes `analyst`.
  Granting and revoking company access requires `admin`.

## Endpoints

| Method | Path                     | Access        | Purpose                                |
| ------ | ------------------------ | ------------- | -------------------------------------- |
| GET    | `/healthz`               | public        | Liveness probe                         |
| POST   | `/api/v1/auth/register`  | public        | Create a user                          |
| POST   | `/api/v1/auth/login`     | public        | Open a session and set the cookie      |
| POST   | `/api/v1/auth/logout`    | session       | Revoke the current session             |
| GET    | `/api/v1/me`             | session       | Current user id, email and role        |
| GET    | `/api/v1/me/companies`   | session       | Company ids the user may read          |
| POST   | `/api/v1/access/grant`   | admin         | Grant a user access to a company       |
| POST   | `/api/v1/access/revoke`  | admin         | Revoke a user's access to a company    |

The schemas are in `contracts/openapi/identity.yaml`.

`/api/v1/me/companies` lives under `/me` on purpose: `/api/v1/companies` belongs to the
company service and is routed there by the gateway.

## Requirements

- Email is normalised to lower case and must parse as a mailbox (max 254 bytes).
- Password must be at least 8 characters and at most 72 bytes (the bcrypt limit).

## Configuration

| Variable                  | Default                                                                        |
| ------------------------- | ------------------------------------------------------------------------------ |
| `IDENTITY_ADDR`           | `:8086`                                                                        |
| `IDENTITY_DATABASE_URL`   | `postgres://identity_svc:identity@localhost:5432/identity_db?sslmode=disable`   |
| `IDENTITY_SESSION_TTL`    | `168h`                                                                         |
| `SESSION_COOKIE_NAME`     | `alfa_session`                                                                 |
| `SESSION_COOKIE_SECURE`   | `false` (set to `true` behind HTTPS)                                           |
| `MIGRATE_ONLY`            | unset (set to `1` to apply migrations and exit)                                |

## Running

Through the stack:

```bash
make infra
make migrate
make dev
```

The service applies its migrations on start, registers itself on `:8086` and is reachable
through the gateway on `:8080`.

## Manual check

```bash
BASE=http://localhost:8080

curl -fsS -X POST "$BASE/api/v1/auth/register" \
  -H 'Content-Type: application/json' \
  -d '{"email":"owner@example.com","password":"secret-pass"}'

curl -fsS -c /tmp/alfa.jar -X POST "$BASE/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"email":"owner@example.com","password":"secret-pass"}'

curl -fsS -b /tmp/alfa.jar "$BASE/api/v1/me"
# {"id":"usr_...","email":"owner@example.com","role":"admin"}

curl -fsS -b /tmp/alfa.jar -X POST "$BASE/api/v1/access/grant" \
  -H 'Content-Type: application/json' \
  -d '{"user_id":"usr_...","company_id":"magnit"}'

curl -fsS -b /tmp/alfa.jar "$BASE/api/v1/me/companies"
# {"items":["magnit"]}

curl -fsS -b /tmp/alfa.jar -X POST "$BASE/api/v1/auth/logout"
```

`make smoke` runs the same scenario against a running stack.

## Tests

```bash
go test -race ./services/identity/...
```

The tests use `MemStore` and mirror the HTTP surface, so they need no database.

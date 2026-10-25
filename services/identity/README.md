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
  login, on a background interval (`IDENTITY_SESSION_PRUNE_INTERVAL`) and rejected on every read.
- Successful logins reset the per-email failure counter. After `IDENTITY_LOGIN_MAX_ATTEMPTS`
  failures inside `IDENTITY_LOGIN_WINDOW` the service answers `429` with `Retry-After`.
- Registration is disabled unless `IDENTITY_REGISTRATION_ENABLED` is true. The demo environment
  enables it so the bootstrap script can create accounts.
- The client may authenticate with the session cookie or with `Authorization: Bearer <token>`.
- The first registered user becomes `admin`; every later registration becomes `analyst`.
  Granting and revoking company access through `/api/v1/access` requires `admin`.
- Membership is the only source of company access. Every role, including `admin`, sees only
  the companies in `company_access`; the admin role does not bypass company checks.
- A membership has a role, `owner` or `member`. The creator of a company becomes its owner.
  Owners can add existing users by email and remove members; members see the roster read-only.
  The last owner cannot be removed. System admins can manage any company.
- A signed-in user can update their email and primary company and change their password.
- Changing the password revokes every other session of the user; the session that performed
  the change stays valid.
- Removing a membership clears the user's primary company when it pointed at that company.

## Endpoints

| Method | Path                     | Access        | Purpose                                |
| ------ | ------------------------ | ------------- | -------------------------------------- |
| GET    | `/healthz`               | public        | Liveness probe                         |
| POST   | `/api/v1/auth/register`  | public*       | Create a user (when enabled)           |
| POST   | `/api/v1/auth/login`     | public        | Open a session and set the cookie      |
| POST   | `/api/v1/auth/logout`    | public        | Revoke the token if present, clear cookie |
| GET    | `/api/v1/me`             | session       | Current user id, email, role and primary company |
| PATCH  | `/api/v1/me`             | session       | Change email and/or primary company    |
| POST   | `/api/v1/me/password`    | session       | Change the password, revoke other sessions |
| GET    | `/api/v1/me/companies`   | session       | Membership ids for the current user    |
| GET    | `/api/v1/me/memberships` | session       | Membership ids and roles               |
| GET    | `/api/v1/companies/{companyId}/members` | member or admin | Company roster |
| POST   | `/api/v1/companies/{companyId}/members` | owner or admin | Add an existing user by email |
| DELETE | `/api/v1/companies/{companyId}/members/{userId}` | owner or admin | Remove a member |
| POST   | `/api/v1/access/grant`   | admin         | Grant a membership with an optional role |
| POST   | `/api/v1/access/revoke`  | admin         | Revoke a membership                   |
| POST   | `/api/v1/internal/memberships` | internal | Record the creator as owner after company creation |

The schemas are in `contracts/openapi/identity.yaml`.

`*` — registration must be enabled for `/auth/register`.

`/api/v1/me/companies` lives under `/me` on purpose: `/api/v1/companies` belongs to the
company service and is routed there by the gateway. It lists memberships and is what the
gateway uses for access checks.

`/api/v1/internal/memberships` is called by the gateway after the company service creates a
company. It is not routed through the public gateway and is only reachable on the internal
network.

## Requirements

- Email is normalised to lower case and must parse as a mailbox (max 254 bytes).
- Password must be at least 8 characters and at most 72 bytes (the bcrypt limit).

## Configuration

| Variable                  | Default                                                                        |
| ------------------------- | ------------------------------------------------------------------------------ |
| `IDENTITY_ADDR`           | `:8086`                                                                        |
| `IDENTITY_DATABASE_URL`   | `postgres://identity_svc:identity@localhost:5432/identity_db?sslmode=disable`   |
| `IDENTITY_SESSION_TTL`    | `168h`                                                                         |
| `IDENTITY_SESSION_PRUNE_INTERVAL` | `1h`                                                                   |
| `IDENTITY_REGISTRATION_ENABLED` | `false` (the demo `.env` sets it to `true`)                            |
| `IDENTITY_LOGIN_MAX_ATTEMPTS` | `5`                                                                        |
| `IDENTITY_LOGIN_WINDOW`   | `15m`                                                                          |
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
  -d '{"user_id":"usr_...","company_id":"magnit","role":"owner"}'

curl -fsS -b /tmp/alfa.jar "$BASE/api/v1/me/memberships"
# {"items":[{"company_id":"magnit","role":"owner"}]}

curl -fsS -b /tmp/alfa.jar "$BASE/api/v1/companies/magnit/members"
curl -fsS -b /tmp/alfa.jar -X POST "$BASE/api/v1/companies/magnit/members" \
  -H 'Content-Type: application/json' \
  -d '{"email":"colleague@example.com"}'

curl -fsS -b /tmp/alfa.jar -X POST "$BASE/api/v1/auth/logout"
```

`make smoke` runs the same scenario against a running stack.

## Tests

```bash
go test -race ./services/identity/...
```

The tests use `MemStore` and mirror the HTTP surface, so they need no database.

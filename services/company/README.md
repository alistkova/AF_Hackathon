# Company service

Owns company profiles and company-related data in `company_db`. Other services must use this HTTP API instead of querying the database directly.

## M0 scope

The company part of M0 provides:

- company list and profile API;
- PostgreSQL schema and migrations;
- CSV seed command;
- health endpoint for Compose and the gateway.

People, payments, groups and company relations are part of M1. The `people` and `company_people` tables exist as a starting schema, but they do not have HTTP endpoints yet.

## API

The OpenAPI contract is [contracts/openapi/company.yaml](../../contracts/openapi/company.yaml). The public contract exposed through the gateway is [contracts/openapi/gateway.yaml](../../contracts/openapi/gateway.yaml).

```text
GET /healthz
GET /api/v1/companies
GET /api/v1/companies/{companyId}
```

List response:

```json
{
  "items": [
    {
      "id": "magnit",
      "name": "Магнит",
      "inn": "5063471190",
      "industry": "Ритейл",
      "region": "Краснодар",
      "description": "Одна из крупнейших розничных сетей России.",
      "created_at": "2026-10-22T10:00:00Z"
    }
  ]
}
```

Profile response is a single company object. Errors use the shared shape:

```json
{
  "code": "company_not_found",
  "message": "Компания не найдена"
}
```

Clients should call the gateway at `http://localhost:8080/api/v1/companies`. The service listens on port `8081` directly only for local debugging and internal calls from insight.

## Data model

`companies`:

- `id` — stable string identifier used by gateway, insight and web;
- `name`, `inn`, `industry`, `region`, `description` — profile fields;
- `created_at` — insertion timestamp.

`people`:

- `id`, `full_name`, `role`, `created_at`;
- reserved for M1, currently without API.

`company_people`:

- `company_id`, `person_id`, `relation`;
- many-to-many relation between companies and people;
- reserved for M1, currently without API.

The service owns all three tables. Do not read or write `company_db` from another service.

## Configuration

| Variable | Default | Meaning |
| --- | --- | --- |
| `COMPANY_ADDR` | `:8081` | HTTP listen address |
| `COMPANY_DATABASE_URL` | `postgres://company_svc:company@localhost:5432/company_db?sslmode=disable` | PostgreSQL connection string |
| `MIGRATE_ONLY` | unset | Set to `1` to apply migrations and exit |

## Local run

From the repository root:

```bash
make infra
make migrate
make seed
make dev
```

`make seed` loads `seed/companies.csv`. To run only the company service:

```bash
COMPANY_DATABASE_URL=postgres://company_svc:company@localhost:5432/company_db?sslmode=disable \
  go run ./services/company/cmd/api
```

Check the service directly:

```bash
curl -s localhost:8081/api/v1/companies
curl -s localhost:8081/api/v1/companies/magnit
```

Check it through the gateway:

```bash
curl -s localhost:8080/api/v1/companies
curl -s localhost:8080/api/v1/companies/magnit
```

## Seed format

The seed command accepts a CSV with a header row:

```csv
id,name,inn,industry,region,description
"magnit","Магнит","5063471190","Ритейл","Краснодар","Одна из крупнейших розничных сетей России."
```

Rules:

- columns are matched by header name, so their order is flexible;
- `id` and `name` are required;
- `inn`, `industry`, `region` and `description` are optional;
- extra columns are ignored;
- a UTF-8 BOM is accepted;
- rows with an empty `id` or `name` stop the import with an error;
- existing companies are updated by `id`.

## Migrations

SQL files live in `services/company/migrations/` and are embedded into the binary. `db.Migrate` applies them in filename order and records versions in `schema_migrations`.

To apply migrations without starting the server:

```bash
MIGRATE_ONLY=1 go run ./services/company/cmd/api
```

## Tests

```bash
go test ./services/company/...
```

The tests cover HTTP handlers with a fake store and CSV parsing, including header reordering, BOM, missing columns and invalid rows.

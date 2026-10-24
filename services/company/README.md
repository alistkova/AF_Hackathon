# Company service

Owns company profiles and company-related data in `company_db`. Other services must use this HTTP API instead of querying the database directly.

## M1 scope

The company part of M1 provides:

- company list and profile API with people and relations;
- company context imported from the hackathon CSV datasets;
- groups, shared people and payment counterparties resolved in PostgreSQL;
- PostgreSQL schema and migrations;
- CSV import command for companies, people links and payments;
- health endpoint for Compose and the gateway.

Identity, sessions and access control live in a separate service. Company data is the source for the company context used by insight and web.

## API

The OpenAPI contract is [contracts/openapi/company.yaml](../../contracts/openapi/company.yaml). The public contract exposed through the gateway is [contracts/openapi/gateway.yaml](../../contracts/openapi/gateway.yaml).

```text
GET /healthz
GET /api/v1/companies
GET /api/v1/companies/{companyId}
GET /api/v1/companies/{companyId}/people
GET /api/v1/companies/{companyId}/relations
```

List response:

```json
{
  "items": [
    {
      "id": "U-1",
      "name": "U-1",
      "inn": "",
      "industry": "Торговля",
      "region": "Москва",
      "description": "",
      "group_id": "G-1",
      "okved_code": "46.90",
      "okved_name": "Торговля",
      "client_type": "Действующий",
      "revenue": "1000000",
      "city": "Москва",
      "products": {"loans": "1", "rko": "0"},
      "attributes": {"org_type": "ООО"},
      "created_at": "2026-10-22T10:00:00Z"
    }
  ]
}
```

Profile response is a company object with `people` and `relations`:

```json
{
  "id": "U-1",
  "name": "U-1",
  "people": [
    {
      "person": {"id": "F-1", "full_name": "F-1", "role": "", "city": "Москва", "region": "Москва", "created_at": "2026-10-22T10:00:00Z"},
      "relation": "linked_person"
    }
  ],
  "relations": [
    {"company_id": "U-2", "company_name": "U-2", "type": "group", "relation": "same_group"},
    {"company_id": "U-3", "company_name": "U-3", "type": "shared_person", "relation": "shared_person", "shared_people": 1},
    {"company_id": "U-4", "company_name": "U-4", "type": "counterparty", "relation": "payment", "direction": "outbound", "amount": 1250000.5, "count": 12}
  ]
}
```

Relations are derived on read:

- `group` — companies with the same non-empty `group_id`;
- `shared_person` — companies that share at least one linked person;
- `counterparty` — companies connected by a payments row, with `direction`, `amount` and `count`.

Errors use the shared shape:

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
- `group_id`, `okved_code`, `okved_name`, `client_type`, `revenue`, `city` — company context;
- `products` — JSONB map of bank products from the dataset (`rko`, `loans`, `deposits`, and so on);
- `attributes` — JSONB map of the remaining dataset attributes (org type, age, workers, FTS flags);
- `created_at` — insertion timestamp.

`people`:

- `id`, `full_name`, `role`, `city`, `region`, `created_at`;
- the hackathon dataset is anonymized, so `full_name` falls back to the synthetic person id.

`company_people`:

- `company_id`, `person_id`, `relation`;
- many-to-many relation between companies and people;
- the same person linked to several companies creates `shared_person` relations.

`payments`:

- `payer_id`, `beneficiary_id`, `amount`, `payment_count`;
- directed payment aggregate between two companies;
- used for counterparty relations.

The service owns all four tables. Do not read or write `company_db` from another service.

## Import

`make seed` imports `seed/companies.csv` and, when present, `seed/people.csv` and `seed/payments.csv`.

To import the hackathon datasets directly:

```bash
go run ./services/company/cmd/seed \
  -file path/to/df_Uid_data_hack.csv \
  -people path/to/df_Fid_Uid_link_hack.csv \
  -payments path/to/payments.csv
```

The companies parser accepts both the demo schema and the hackathon schema:

- demo columns: `id`, `name`, `inn`, `industry`, `region`, `description`;
- hackathon columns: `SYNTHETIC_U`, `MAINOKVED_CODE`, `MAINOKVED_NAME`, `REGION`, `CITY`, `GROUP_ID`, `REVENUE_COMPANY`, `CL_TYPE`, `CL_PRODUCT_*_MARK`, `ORG_TYPE`, and the other attributes listed in the case;
- when `name` is absent, the synthetic company id is used as the display name;
- columns are matched by header name, so their order is flexible and a UTF-8 BOM is accepted;
- extra columns are ignored;
- existing rows are updated by their identifiers;
- payment rows that reference unknown companies are skipped, so noisy data does not break the import.

People links use `SYNTHETIC_F` + `SYNTHETIC_U` (or `person_id` + `company_id`). Payments use `PAYER`, `BENEFICIARY`, `PAY_AMT`, `PAY_CNT` (or snake_case equivalents). Amounts and counts tolerate spaces and comma decimal separators.

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

`make seed` loads the demo CSVs. To run only the company service:

```bash
COMPANY_DATABASE_URL=postgres://company_svc:company@localhost:5432/company_db?sslmode=disable \
  go run ./services/company/cmd/api
```

Check the service directly:

```bash
curl -s localhost:8081/api/v1/companies
curl -s localhost:8081/api/v1/companies/magnit
curl -s localhost:8081/api/v1/companies/magnit/relations
```

Check it through the gateway:

```bash
curl -s localhost:8080/api/v1/companies
curl -s localhost:8080/api/v1/companies/magnit
curl -s localhost:8080/api/v1/companies/magnit/relations
```

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

The tests cover HTTP handlers with a fake store and CSV parsing, including header reordering, BOM, demo and hackathon schemas, people links, payments, missing columns and invalid rows.

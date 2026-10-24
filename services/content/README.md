# Content service

Owns documents, news, chunks and their text in `content_db`. Other services use this HTTP
API; nobody reads `content_db` directly.

## Pipeline

Every document goes through the same path:

1. **Normalise** — collapse whitespace, trim lines, derive a stable id when the caller
   does not supply one.
2. **Deduplicate** — the canonical source URL (`NormalizeURL`) and a SHA-256 hash over the
   lowercased title and body (`ContentHash`) are unique. A republished article that arrives
   with a new id but the same URL or the same text is reported as `duplicate` and only
   bumps `seen_count`; nothing is inserted.
3. **Chunk** — the body is split into ~1200 character chunks with 200 characters of
   overlap, preferring paragraph and sentence boundaries. Chunks are stored in `chunks`
   with a stable `ordinal`, so re-ingesting replaces them instead of duplicating.
4. **Search** — `documents.search_vector` is a generated `tsvector` over title and body
   with the `russian` configuration, backed by a GIN index. Search ranks with `ts_rank_cd`.

`NormalizeURL` lowercases the host, drops `www.`, default ports, fragments and tracking
parameters (`utm_*`, `fbclid`, `gclid`, …), sorts the remaining query parameters and
removes trailing slashes. That makes `https://www.rbc.ru/news/1/?utm_source=tg` and
`https://rbc.ru/news/1` the same document.

## API

| Method | Path                                   | Purpose                                  |
| ------ | -------------------------------------- | ---------------------------------------- |
| GET    | `/healthz`                             | Liveness probe                           |
| GET    | `/api/v1/news`                         | Filtered, paginated document list        |
| GET    | `/api/v1/news/{id}`                    | One document by `news_id` or `document_id` |
| GET    | `/api/v1/search?q=`                    | Ranked full-text search                  |
| POST   | `/api/v1/documents`                    | Ingest JSON documents                    |
| POST   | `/api/v1/documents/pdf`                | Upload a PDF, extract text and ingest    |
| GET    | `/api/v1/documents/{id}/chunks`        | Chunks of one document                   |
| POST   | `/api/v1/rag/chunks`                   | Upsert agent-embedded chunks             |
| POST   | `/api/v1/rag/chunks/search`            | Vector search over agent chunks          |
| POST   | `/api/v1/rag/chunks/list`              | List agent chunks by metadata            |
| GET    | `/api/v1/rag/chunks/count`             | Count agent chunks                       |
| DELETE | `/api/v1/rag/chunks`                   | Drop agent chunks                        |

The contract is [contracts/openapi/content.yaml](../../contracts/openapi/content.yaml).
`/api/v1/news` keeps the bare-array shape the AI team expects and reports the total in the
`X-Total-Count` header.

### Filters

`kind`, `sector`, `tag`, `source`, `from`, `to`, `limit` (1..200, default 50), `offset`.
`from`/`to` accept `2006-01-02` or RFC3339. `sector` and `tag` match array membership
exactly (`target_sectors @> ARRAY[...]`).

### Ingest

```bash
curl -X POST localhost:8082/api/v1/documents \
  -H 'Content-Type: application/json' \
  -d '{"items":[{"news_id":"n1","title":"Ставка","full_text":"...","source_url":"https://rbc.ru/a"}]}'
```

Response:

```json
{"items":[{"status":"created","document_id":"n1","news_id":"n1","chunks":3}]}
```

`status` is `created`, `updated` (same document id, new content) or `duplicate` (another
document already has this URL or this text hash).

### PDF

```bash
curl -X POST localhost:8082/api/v1/documents/pdf \
  -F 'file=@material.pdf' \
  -F 'source_name=Альфа-Банк' \
  -F 'published_at=2026-08-31' \
  -F 'sectors=Банки,Ритейл' \
  -F 'tags=ставка,макро'
```

The document id is `pdf_<sha256 prefix>`, so uploading the same file twice updates the
existing row. Optional fields: `title`, `source_name`, `source_url`, `published_at`,
`tags`, `sectors` (comma-separated). When `title` is omitted the first non-empty line of
the extracted text is used.

Text extraction runs `pdftotext` from **poppler-utils**. The Docker image installs it; on a
host without poppler the endpoint answers `422 pdf_unavailable` and the rest of the service
keeps working. Override the binary with `PDFTOTEXT_BIN`.

## Storage

Raw PDFs are written through the `Storage` interface. The default backend is local disk
under `CONTENT_DATA_DIR` (`/data/content` in Compose, backed by the `contentdata` volume),
sharded as `pdf/<first two hex>/<sha256>.pdf`. Keys are validated against traversal.
Swapping in MinIO means implementing the same three methods.

## Data model

- `documents` — one row per document: profile fields, `kind` (`news`/`pdf`), `content_hash`,
  `canonical_url`, `object_key`, `byte_size`, `chunk_count`, `seen_count`, `last_seen_at`
  and the generated `search_vector`.
- `chunks` — `document_id`, `ordinal`, `content`, `char_count`; cascade on delete.
- `rag_chunks` — separate table owned for the agent's RAG corpus (384-dim embeddings,
  `company_id`/`sector` metadata). It is fed by the agent through the `/api/v1/rag/*`
  endpoints and is not touched by the document pipeline.

## Configuration

| Variable                | Default                                                                    |
| ----------------------- | -------------------------------------------------------------------------- |
| `CONTENT_ADDR`          | `:8082`                                                                    |
| `CONTENT_DATABASE_URL`  | `postgres://content_svc:content@localhost:5432/content_db?sslmode=disable`  |
| `CONTENT_DATA_DIR`      | `./.data/content`                                                          |
| `PDFTOTEXT_BIN`         | `pdftotext`                                                                |
| `MIGRATE_ONLY`          | unset (set to `1` to apply migrations and exit)                            |

## Seed

```bash
go run ./services/content/cmd/seed -file seed/news/sample.json
```

Reports `created`, `updated`, `duplicates` and the number of stored chunks.

## Tests

```bash
go test -race ./services/content/...
```

Covers URL canonicalisation, hashing, chunking, filters, search plumbing, PDF validation
and storage key safety. PDF text extraction itself needs `pdftotext`; the test suite treats
its absence as a valid environment.

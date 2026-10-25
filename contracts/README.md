# Contracts

OpenAPI files describe request/response bodies. SSE payloads are described by JSON Schema files in `events/`.

## SSE framing

Each SSE message is:

```text
id: 4
event: step
data: {"type":"step","job_id":"...","seq":4,"ts":"2026-10-20T12:00:00Z","name":"retrieving","status":"done","message":"Найдено 3 источника"}

```

Rules:

- `id` is the monotonic `seq` for the job.
- `event` equals `data.type`.
- `result` and `error` are terminal. The server closes the stream after them.
- Heartbeats are comments: `: heartbeat`.
- Clients may reconnect with `Last-Event-ID`; the server replays later events.

## Agent boundary

`services/agent` is stateless. Insight owns retrieval, persistence and retries. The agent receives evidence with `source_id` values and returns `sources` containing only those IDs.

## Feed

`GET /api/v1/feed?company_id=...` is assembled by insight from content-service news. Each item carries a `relevance` score from 0 to 1 and `reasons` such as `Упоминание компании` or `Отрасль: Торговля`. Ranking uses the company name, industry mapped to news `target_sectors`, region and publication date. Insight does not read `content_db`; it calls the public news API.

The feed can be filtered by repeatable `source`, `tag` and `sector` parameters, by `q`, and by `from` / `to` dates. Filters are applied after ranking. The response includes `facets` with unfiltered counts for sources, tags and sectors.

## News API format

`GET /api/v1/news` returns the array shape requested by the AI team:

```json
{
  "news_id": "news_2026_09_11_001",
  "title": "Банк России сохранил ключевую ставку на уровне 14% годовых",
  "published_at": "2026-09-11T13:30:00Z",
  "source_name": "РБК",
  "source_url": "https://www.rbc.ru/finances/...",
  "full_text": "Совет директоров Банка России...",
  "tags": ["макроэкономика", "ключевая ставка"],
  "target_sectors": ["Финансы", "Строительство"]
}
```

See `contracts/openapi/content.yaml` and `contracts/examples/news.json`.

## Document pipeline

`services/content` normalises every document, rejects duplicates by canonical URL and by a
SHA-256 hash of the normalised title and body, and splits the body into overlapping chunks
stored in `chunks`. `documents.search_vector` is a generated `tsvector` over title and body
with the `russian` configuration, so `GET /api/v1/search?q=` ranks with `ts_rank_cd`.

Raw PDFs go through `POST /api/v1/documents/pdf`, are stored under `CONTENT_DATA_DIR` and
are turned into text by `pdftotext`. Both ingestion endpoints return a per-document status:
`created`, `updated` or `duplicate`.

## Identity API

`services/identity` owns users, sessions and company access. Cookies and bearer tokens are
described in `contracts/openapi/identity.yaml`. The gateway forwards `/api/v1/auth`,
`/api/v1/me` and `/api/v1/access` to it; `/api/v1/companies` stays with the company service.

## RAG chunk API

`services/content` owns the `rag_chunks` table in `content_db`. The agent computes embeddings and calls the content service, so it never connects to `content_db`.

- `POST /api/v1/rag/chunks` upserts chunks keyed by `source`, `page` and `chunk`.
- `POST /api/v1/rag/chunks/search` runs a cosine similarity search with `company_id`, `sector`, `published_after` and `published_before` filters.
- `POST /api/v1/rag/chunks/list` lists chunks by the same metadata filters.
- `GET /api/v1/rag/chunks/count` and `DELETE /api/v1/rag/chunks` report and clear the table.

Embeddings are 384-dimensional (`sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2`). The schemas are in `contracts/openapi/content.yaml`.

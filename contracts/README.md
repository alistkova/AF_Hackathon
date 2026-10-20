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

# API

Справочник публичного API. Все запросы идут через gateway: `http://localhost:8080` (в Docker-демо web проксирует тот же путь с `http://localhost:3000`). Внутренние адреса сервисов (`127.0.0.1:8081`, `:8082`, `:8083`, `:8086`, `:8090`) опубликованы только для отладки и не предназначены для клиентов.

- Контракты: [`contracts/openapi/*.yaml`](../contracts/openapi/), SSE-схемы: [`contracts/events/`](../contracts/events/).
- Типы фронтенда генерируются из `contracts/openapi/gateway.yaml` командой `make gen`.
- `X-Request-ID` принимается и возвращается gateway; по нему ищутся логи.

## Содержание

- [Общие правила](#общие-правила)
- [Аутентификация](#аутентификация)
- [Профиль](#профиль)
- [Компании](#компании)
- [Участники компании](#участники-компании)
- [Гранты доступа](#гранты-доступа)
- [Новости, документы, поиск](#новости-документы-поиск)
- [Лента](#лента)
- [Разбор события и SSE](#разбор-события-и-sse)
- [Внутренние RAG-эндпоинты](#внутренние-rag-эндпоинты)
- [Сквозной сценарий](#сквозной-сценарий)

## Общие правила

**Формат ошибок.** Единый JSON:

```json
{ "code": "company_not_found", "message": "Компания не найдена" }
```

**Коды ответов.** `400` — некорректный запрос, `401` — нет или протухла сессия, `403` — недостаточно прав, `404` — объект не найден или недоступен пользователю, `409` — конфликт (например, email занят), `502`/`503` — недоступен нижележащий сервис.

**Аутентификация.** Cookie `alfa_session` (ставится при логине) или заголовок `Authorization: Bearer <token>`. Токен — то же значение, что в cookie.

**Пагинация.** Списки новостей и ленты принимают `limit` и `offset`. Лента возвращает `total` в теле, новости — в заголовке `X-Total-Count`.

**Даты.** `2006-01-02` или RFC3339 (`2026-10-26T12:00:00Z`). Для `to` без времени берётся конец дня.

**Публичные маршруты.** `GET /api/v1/news*` и `GET /api/v1/search` доступны без сессии. Всё остальное требует входа.

## Аутентификация

### `POST /api/v1/auth/register`

```bash
curl -s -X POST localhost:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"owner@example.com","password":"secret-pass"}'
```

`201` — пользователь создан, `409` — email уже занят, `403` — регистрация выключена (`IDENTITY_REGISTRATION_ENABLED=false`). Первый зарегистрированный пользователь получает роль `admin`, последующие — `analyst`. Пароль: от 8 символов, до 72 байт.

### `POST /api/v1/auth/login`

```bash
curl -s -c /tmp/alfa.jar -X POST localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"owner@example.com","password":"secret-pass"}'
```

`200` и `Set-Cookie: alfa_session=...`. `401` — неверные данные, `429` — превышен лимит попыток (`IDENTITY_LOGIN_MAX_ATTEMPTS` за `IDENTITY_LOGIN_WINDOW`).

Ответ:

```json
{ "id": "usr_...", "email": "owner@example.com", "role": "analyst", "primary_company_id": "" }
```

### `POST /api/v1/auth/logout`

Требует сессию. Удаляет текущую сессию и очищает cookie. `204`.

## Профиль

| Метод | Путь | Назначение |
| --- | --- | --- |
| `GET` | `/api/v1/me` | Текущий пользователь |
| `PATCH` | `/api/v1/me` | Email и основная компания |
| `POST` | `/api/v1/me/password` | Смена пароля (отзывает остальные сессии) |
| `GET` | `/api/v1/me/companies` | Id компаний, доступных пользователю |
| `GET` | `/api/v1/me/memberships` | Членства с ролями |

```bash
curl -s -b /tmp/alfa.jar localhost:8080/api/v1/me
curl -s -b /tmp/alfa.jar -X PATCH localhost:8080/api/v1/me \
  -H 'Content-Type: application/json' \
  -d '{"email":"new@example.com","primary_company_id":"magnit"}'
curl -s -b /tmp/alfa.jar -X POST localhost:8080/api/v1/me/password \
  -H 'Content-Type: application/json' \
  -d '{"current_password":"secret-pass","new_password":"next-secret"}'
```

`GET /api/v1/me/memberships` → `[{"company_id":"magnit","role":"owner"}]`.

## Компании

| Метод | Путь | Доступ |
| --- | --- | --- |
| `GET` | `/api/v1/companies` | Членство; фильтруется по доступным компаниям |
| `POST` | `/api/v1/companies` | Любой авторизованный; создатель становится владельцем |
| `GET` | `/api/v1/companies/{id}` | Членство (или admin) |
| `PUT` | `/api/v1/companies/{id}` | Владелец компании или admin |
| `GET` | `/api/v1/companies/{id}/people` | Членство |
| `GET` | `/api/v1/companies/{id}/relations` | Членство |

```bash
curl -s -b /tmp/alfa.jar localhost:8080/api/v1/companies
curl -s -b /tmp/alfa.jar localhost:8080/api/v1/companies/magnit
curl -s -b /tmp/alfa.jar localhost:8080/api/v1/companies/magnit/relations

curl -s -b /tmp/alfa.jar -X POST localhost:8080/api/v1/companies \
  -H 'Content-Type: application/json' \
  -d '{"name":"ООО «Ромашка»","inn":"7701234567","industry":"Ритейл","region":"Москва"}'
```

`GET /api/v1/companies` → `{"items":[Company]}` — только доступные компании, сортировка по названию. `GET /api/v1/companies/{id}` → `Company` с `people` и `relations`. Чужая компания → `404`.

`Company`:

```json
{
  "id": "magnit",
  "name": "Магнит",
  "inn": "5063471190",
  "industry": "Ритейл",
  "region": "Краснодарский край",
  "description": "Розничная сеть",
  "city": "Краснодар",
  "revenue": "2600000000",
  "okved_code": "47.11",
  "okved_name": "Торговля розничная",
  "products": { "расчётный счёт": "да" },
  "attributes": {},
  "created_at": "2026-10-25T10:00:00Z"
}
```

`PUT` принимает `name`, `inn`, `industry`, `region`, `city`, `revenue`, `description`, `okved_code`, `okved_name`, `products`, `attributes`. `POST` возвращает `201` и компанию; вызывающая сторона автоматически становится владельцем, gateway дополнительно дергает identity для членства.

## Участники компании

| Метод | Путь | Доступ |
| --- | --- | --- |
| `GET` | `/api/v1/companies/{id}/members` | Владелец компании или admin |
| `POST` | `/api/v1/companies/{id}/members` | Владелец компании или admin |
| `PATCH` | `/api/v1/companies/{id}/members/{userId}` | Владелец компании или admin |
| `DELETE` | `/api/v1/companies/{id}/members/{userId}` | Владелец компании или admin |

```bash
curl -s -b /tmp/alfa.jar localhost:8080/api/v1/companies/magnit/members
curl -s -b /tmp/alfa.jar -X POST localhost:8080/api/v1/companies/magnit/members \
  -H 'Content-Type: application/json' -d '{"email":"colleague@example.com"}'
curl -s -b /tmp/alfa.jar -X PATCH localhost:8080/api/v1/companies/magnit/members/usr_123 \
  -H 'Content-Type: application/json' -d '{"role":"member"}'
curl -s -b /tmp/alfa.jar -X DELETE localhost:8080/api/v1/companies/magnit/members/usr_123
```

Ответ — `[{"user_id":"usr_...","email":"...","role":"owner|member"}]`. Последнего владельца нельзя удалить или понизить. При удалении участника его `primary_company_id` очищается, если указывал на эту компанию.

## Гранты доступа

Только `admin`.

```bash
curl -s -b /tmp/alfa.jar -X POST localhost:8080/api/v1/access/grant \
  -H 'Content-Type: application/json' \
  -d '{"user_id":"usr_123","company_id":"magnit","role":"owner"}'

curl -s -b /tmp/alfa.jar -X POST localhost:8080/api/v1/access/revoke \
  -H 'Content-Type: application/json' \
  -d '{"user_id":"usr_123","company_id":"magnit"}'
```

`role` опционален (`member` по умолчанию). Операции идемпотентны: повторный grant обновляет роль.

## Новости, документы, поиск

### `GET /api/v1/news`

Публичный. Формат AI-команды.

| Параметр | Значение |
| --- | --- |
| `kind` | `news` или `document` |
| `sector` | отрасль, повторяемый |
| `tag` | тег, повторяемый |
| `source` | источник, повторяемый |
| `from`, `to` | период публикации |
| `limit`, `offset` | пагинация (по умолчанию 50, максимум 200) |

```bash
curl -s 'localhost:8080/api/v1/news?kind=news&sector=Торговля&limit=2'
curl -s 'localhost:8080/api/v1/news?tag=логистика&from=2026-09-01' -D - -o /dev/null | grep -i x-total-count
```

```json
[
  {
    "document_id": "doc_...",
    "news_id": "n_...",
    "title": "Магнит увеличивает инвестиции в логистику на юге России",
    "published_at": "2026-10-20T09:00:00Z",
    "source_name": "Ведомости",
    "source_url": "https://...",
    "full_text": "...",
    "tags": ["ритейл", "логистика"],
    "target_sectors": ["Торговля", "Логистика"]
  }
]
```

`GET /api/v1/news/{id}` — один документ, `404` если нет.

### `POST /api/v1/documents`

Только `admin`. JSON-массив новостей в том же формате.

```bash
curl -s -b /tmp/alfa.jar -X POST localhost:8080/api/v1/documents \
  -H 'Content-Type: application/json' \
  -d '{"items":[{"news_id":"n100","title":"Ключевая ставка","full_text":"...","tags":["ставка"],"target_sectors":["Финансы"]}]}'
```

Ответ: `{"items":[{"status":"created|updated|duplicate","document_id":"doc_...","news_id":"n100","chunks":3}]}`. Дубликаты отсекаются по каноническому URL и по хешу содержимого: повторный документ получает `status=duplicate`, счётчик `seen_count` увеличивается.

### `POST /api/v1/documents/pdf`

Только `admin`, `multipart/form-data`.

```bash
curl -s -b /tmp/alfa.jar -X POST localhost:8080/api/v1/documents/pdf \
  -F 'file=@analytics.pdf' -F 'source_name=Альфа-Банк'
```

Текст извлекается `pdftotext`; файл сохраняется в volume `contentdata`.

### `GET /api/v1/documents/{id}/chunks`

Только `admin`. Возвращает нарезку документа (1200 символов, перекрытие 200).
Ответ: `{"items":[{"ordinal":0,"content":"первый чанк…"}]}`.

### `GET /api/v1/search`

Публичный полнотекстовый поиск по новостям и PDF.

```bash
curl -s 'localhost:8080/api/v1/search?q=ключевая%20ставка&limit=5'
```

Ответ: `{"items":[{...NewsItem, "rank":0.83}], "total":12}`; `X-Total-Count` дублирует `total`. Принимает те же фильтры, что и `/api/v1/news`.

## Лента

### `GET /api/v1/feed`

Требует сессию и доступ к компании.

| Параметр | Обязательный | Описание |
| --- | --- | --- |
| `company_id` | да | Компания |
| `q` | нет | Поиск по заголовку, тексту, источнику, тегам, отраслям (до 200 символов) |
| `source`, `tag`, `sector` | нет | Повторяемые или через запятую |
| `from`, `to` | нет | Период публикации |
| `sort` | нет | `relevance` (по умолчанию) или `date` |
| `limit` | нет | 1..50, по умолчанию 30 |
| `offset` | нет | Смещение |

```bash
curl -s -b /tmp/alfa.jar 'localhost:8080/api/v1/feed?company_id=magnit&limit=3'
curl -s -b /tmp/alfa.jar 'localhost:8080/api/v1/feed?company_id=magnit&tag=логистика&sort=date&q=Магнит'
```

```json
{
  "items": [
    {
      "event_id": "n_123",
      "company_id": "magnit",
      "title": "Магнит увеличивает инвестиции в логистику на юге России",
      "summary": "Короткая выжимка до 240 символов…",
      "source": "Ведомости",
      "url": "https://...",
      "published_at": "2026-10-20T09:00:00Z",
      "relevance": 0.71,
      "reasons": ["Упоминание компании", "Отрасль: Торговля"],
      "tags": ["ритейл", "логистика"],
      "target_sectors": ["Торговля", "Логистика"]
    }
  ],
  "total": 4,
  "facets": {
    "sources": [{ "value": "Ведомости", "count": 1 }],
    "tags": [{ "value": "логистика", "count": 2 }],
    "sectors": [{ "value": "Торговля", "count": 3 }]
  }
}
```

`total` — после фильтров, `facets` — по несфильтрованному окну из 200 новостей. `relevance` — нормированная оценка (см. README, раздел «Лента и ранжирование»); новости без совпадений получают `0.1` и причину «Общая повестка».

Недоступная компания → `404 company_not_found`; content недоступен и кэша нет → `502 content_unavailable`.

## Разбор события и SSE

### `POST /api/v1/insights/jobs`

Требует сессию и доступ к компании.

```bash
curl -s -b /tmp/alfa.jar -X POST localhost:8080/api/v1/insights/jobs \
  -H 'Content-Type: application/json' \
  -d '{"company_id":"magnit","event":{"id":"news-001","title":"Магнит расширяет логистику"}}'
```

`202 Accepted`:

```json
{
  "job_id": "job_...",
  "company_id": "magnit",
  "status": "queued",
  "events_url": "/api/v1/insights/jobs/job_.../events",
  "result_url": "/api/v1/insights/jobs/job_..."
}
```

Опционально можно передать `evidence` — массив `{source_id,title,url,snippet,published_at}`. Если он пуст, insight подставит само событие как единственный источник.

### `GET /api/v1/insights/jobs/{jobId}`

Текущее состояние: `job_id`, `company_id`, `event_id`, `status` (`queued|running|done|failed`), `created_at`, `events_url`, `insight` (когда готово), `error` (когда упало).

### `GET /api/v1/insights/jobs/{jobId}/events`

SSE-поток. Терминальные события — `result` и `error`.

```bash
curl -N -b /tmp/alfa.jar localhost:8080/api/v1/insights/jobs/job_.../events
```

```text
id: 1
event: status
data: {"type":"status","job_id":"job_...","seq":1,"status":"running","ts":"..."}

id: 2
event: step
data: {"type":"step","job_id":"job_...","seq":2,"name":"retrieving","status":"running","message":"Ищем связанные новости и документы","ts":"..."}

id: 3
event: step
data: {"type":"step","job_id":"job_...","seq":3,"name":"retrieving","status":"done","message":"Ищем связанные новости и документы","ts":"..."}

id: 9
event: result
data: {"type":"result","job_id":"job_...","seq":9,"insight":{...},"ts":"..."}
```

Правила фрейминга:

- `id` — монотонный `seq`, `event` совпадает с `data.type`;
- после `result`/`error` сервер закрывает поток;
- heartbeat — комментарий `: heartbeat`;
- переподключение с `Last-Event-ID: <seq>` досылает пропущенные события.

Шаги: `planning`, `enriching`, `retrieving`, `ranking`, `analyzing`, `validating`. Статус шага — `running` или `done`.

`Insight` в событии `result`:

```json
{
  "summary": "Магнит: Магнит расширяет логистику",
  "why_it_matters": "Событие влияет на операционную эффективность…",
  "facts": [
    { "text": "Компания расширяет логистику на юге России.", "source_ids": ["news-001"], "confidence": "high" }
  ],
  "hypotheses": [
    { "text": "Эффект проявится в следующем квартале.", "source_ids": ["news-001"], "confidence": "low" }
  ],
  "structured_risks": [
    { "text": "Рост затрат может ограничить денежный поток.", "source_ids": ["news-001"], "confidence": "medium" }
  ],
  "structured_opportunities": [
    { "text": "Усиление позиций на ключевом рынке.", "source_ids": ["news-001"], "confidence": "medium" }
  ],
  "bank_view": { "used": false, "text": "", "source_ids": [] },
  "ai_view": { "text": "Вывод AI: сигнал умеренно позитивный…", "source_ids": ["news-001"] },
  "risks": ["Рост затрат может ограничить свободный денежный поток."],
  "opportunities": ["Усиление позиций на ключевом рынке."],
  "bank_position": "",
  "ai_conclusion": "Вывод AI: сигнал умеренно позитивный…",
  "sources": [{ "source_id": "news-001", "title": "Магнит расширяет логистику", "url": "https://..." }],
  "actions": ["Проверить финансовые показатели компании."],
  "confidence": 0.72,
  "disclaimer": "Не является инвестиционной рекомендацией",
  "meta": { "model": "stub", "prompt_version": "v1" }
}
```

`bank_view.used=false` и пустой `bank_position` означают, что аналитики Альфа-Банка в evidence нет — UI в этом случае не показывает блок «Позиция банка». Legacy-поля `risks`, `opportunities`, `bank_position`, `ai_conclusion` сохранены для обратной совместимости; клиентам следует предпочитать `structured_*`, `bank_view`, `ai_view`.

`meta` может содержать `degraded: true` и `validation_issues: [...]`, если модель не ответила корректно за две попытки.

## Внутренние RAG-эндпоинты

Используются только агентом, наружу не публикуются. Живут на content-сервисе (`http://content:8082`), не через gateway.

| Метод | Путь | Назначение |
| --- | --- | --- |
| `POST` | `/api/v1/rag/chunks` | Upsert чанков с эмбеддингами |
| `POST` | `/api/v1/rag/chunks/search` | Семантический поиск с фильтрами `company_id`, `sector`, `published_at` |
| `POST` | `/api/v1/rag/chunks/list` | Список чанков по метаданным |
| `GET` | `/api/v1/rag/chunks/count` | Количество чанков |
| `DELETE` | `/api/v1/rag/chunks` | Очистка таблицы |

Схема таблицы: `content_db.rag_chunks`, `embedding vector(384)`, HNSW-индекс `vector_cosine_ops`. Агент не открывает `content_db` напрямую.

## Сквозной сценарий

```bash
BASE=http://localhost:8080
JAR=/tmp/alfa.jar

# 1. Вход демо-клиентом
curl -s -c $JAR -X POST $BASE/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"demo-client@alfa.local","password":"demo-password"}'

# 2. Доступные компании
curl -s -b $JAR $BASE/api/v1/companies | head -c 300

# 3. Лента по Магниту
curl -s -b $JAR "$BASE/api/v1/feed?company_id=magnit&limit=2" | head -c 500

# 4. Поставить задачу разбора
JOB=$(curl -s -b $JAR -X POST $BASE/api/v1/insights/jobs \
  -H 'Content-Type: application/json' \
  -d '{"company_id":"magnit","event":{"id":"news-001","title":"Магнит расширяет логистику"}}' \
  | python3 -c 'import json,sys; print(json.load(sys.stdin)["job_id"])')

# 5. Прочитать поток до терминального события
curl -sN -b $JAR "$BASE/api/v1/insights/jobs/$JOB/events" | sed -n '1,40p'

# 6. Результат после завершения
curl -s -b $JAR "$BASE/api/v1/insights/jobs/$JOB" | head -c 800
```

Ошибки и лимиты описаны в [Общих правилах](#общие-правила), доменные детали — в README сервисов ([company](../services/company/README.md), [content](../services/content/README.md), [identity](../services/identity/README.md), [agent](../services/agent/README.md)).

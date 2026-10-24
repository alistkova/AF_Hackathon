ALTER TABLE documents
    ADD COLUMN IF NOT EXISTS kind text NOT NULL DEFAULT 'news',
    ADD COLUMN IF NOT EXISTS content_hash text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS canonical_url text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS object_key text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS mime_type text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS byte_size bigint NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS chunk_count integer NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS seen_count integer NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS last_seen_at timestamptz NOT NULL DEFAULT now();

CREATE UNIQUE INDEX IF NOT EXISTS documents_content_hash_idx
    ON documents (content_hash) WHERE content_hash <> '';

CREATE UNIQUE INDEX IF NOT EXISTS documents_canonical_url_idx
    ON documents (canonical_url) WHERE canonical_url <> '';

ALTER TABLE documents
    ADD COLUMN IF NOT EXISTS search_vector tsvector
    GENERATED ALWAYS AS (
        to_tsvector('russian', coalesce(title, '') || ' ' || coalesce(full_text, ''))
    ) STORED;

CREATE INDEX IF NOT EXISTS documents_search_idx ON documents USING gin (search_vector);
CREATE INDEX IF NOT EXISTS documents_kind_published_idx ON documents (kind, published_at DESC NULLS LAST);

ALTER TABLE chunks
    ADD COLUMN IF NOT EXISTS char_count integer NOT NULL DEFAULT 0;

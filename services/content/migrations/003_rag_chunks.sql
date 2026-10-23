CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS rag_chunks (
    id bigserial PRIMARY KEY,
    content text NOT NULL,
    source text NOT NULL DEFAULT '',
    page integer NOT NULL DEFAULT 0,
    chunk integer NOT NULL DEFAULT 0,
    company_id text NOT NULL DEFAULT '',
    sector text NOT NULL DEFAULT '',
    published_at date,
    embedding vector(384) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (source, page, chunk)
);

CREATE INDEX IF NOT EXISTS rag_chunks_company_idx ON rag_chunks (company_id, sector, published_at DESC);
CREATE INDEX IF NOT EXISTS rag_chunks_embedding_idx ON rag_chunks USING hnsw (embedding vector_cosine_ops);

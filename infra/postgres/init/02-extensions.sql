\connect content_db
CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

\connect insight_db
CREATE EXTENSION IF NOT EXISTS pg_trgm;

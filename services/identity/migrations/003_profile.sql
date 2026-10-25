ALTER TABLE users
    ADD COLUMN IF NOT EXISTS primary_company_id text NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS users_primary_company_id_idx ON users (primary_company_id);

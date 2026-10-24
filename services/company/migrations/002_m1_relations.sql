ALTER TABLE companies
    ADD COLUMN IF NOT EXISTS group_id text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS okved_code text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS okved_name text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS client_type text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS revenue text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS city text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS products jsonb NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS attributes jsonb NOT NULL DEFAULT '{}'::jsonb;

CREATE INDEX IF NOT EXISTS companies_group_id_idx ON companies (group_id) WHERE group_id <> '';

ALTER TABLE people
    ADD COLUMN IF NOT EXISTS city text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS region text NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS company_people_person_idx ON company_people (person_id);

CREATE TABLE IF NOT EXISTS payments (
    payer_id text NOT NULL REFERENCES companies (id) ON DELETE CASCADE,
    beneficiary_id text NOT NULL REFERENCES companies (id) ON DELETE CASCADE,
    amount numeric(20, 2) NOT NULL DEFAULT 0,
    payment_count bigint NOT NULL DEFAULT 0,
    PRIMARY KEY (payer_id, beneficiary_id)
);

CREATE INDEX IF NOT EXISTS payments_beneficiary_idx ON payments (beneficiary_id);

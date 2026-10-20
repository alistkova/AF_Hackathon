CREATE TABLE IF NOT EXISTS companies (
    id text PRIMARY KEY,
    name text NOT NULL,
    inn text NOT NULL DEFAULT '',
    industry text NOT NULL DEFAULT '',
    region text NOT NULL DEFAULT '',
    description text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS companies_name_idx ON companies (name);

CREATE TABLE IF NOT EXISTS people (
    id text PRIMARY KEY,
    full_name text NOT NULL,
    role text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS company_people (
    company_id text NOT NULL REFERENCES companies (id) ON DELETE CASCADE,
    person_id text NOT NULL REFERENCES people (id) ON DELETE CASCADE,
    relation text NOT NULL DEFAULT '',
    PRIMARY KEY (company_id, person_id)
);

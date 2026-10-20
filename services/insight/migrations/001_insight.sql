CREATE TABLE IF NOT EXISTS analysis_jobs (
    id text PRIMARY KEY,
    company_id text NOT NULL,
    event_id text NOT NULL,
    status text NOT NULL,
    insight jsonb,
    error text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS analysis_events (
    job_id text NOT NULL REFERENCES analysis_jobs (id) ON DELETE CASCADE,
    seq bigint NOT NULL,
    type text NOT NULL,
    payload jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (job_id, seq)
);

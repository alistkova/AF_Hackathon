ALTER TABLE documents ADD COLUMN IF NOT EXISTS news_id text;
ALTER TABLE documents ADD COLUMN IF NOT EXISTS source_name text NOT NULL DEFAULT '';
ALTER TABLE documents ADD COLUMN IF NOT EXISTS source_url text NOT NULL DEFAULT '';
ALTER TABLE documents ADD COLUMN IF NOT EXISTS full_text text NOT NULL DEFAULT '';
ALTER TABLE documents ADD COLUMN IF NOT EXISTS tags text[] NOT NULL DEFAULT '{}';
ALTER TABLE documents ADD COLUMN IF NOT EXISTS target_sectors text[] NOT NULL DEFAULT '{}';

UPDATE documents SET news_id = id WHERE news_id IS NULL;
UPDATE documents SET source_name = source WHERE source_name = '' AND source <> '';
UPDATE documents SET source_url = url WHERE source_url = '' AND url <> '';
UPDATE documents SET full_text = body WHERE full_text = '' AND body <> '';

ALTER TABLE documents ALTER COLUMN news_id SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS documents_news_id_idx ON documents (news_id);

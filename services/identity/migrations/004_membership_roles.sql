ALTER TABLE company_access
    ADD COLUMN IF NOT EXISTS role text NOT NULL DEFAULT 'member';

ALTER TABLE company_access
    DROP CONSTRAINT IF EXISTS company_access_role_check;
ALTER TABLE company_access
    ADD CONSTRAINT company_access_role_check CHECK (role IN ('owner', 'member'));

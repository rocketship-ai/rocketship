-- Add audit fields to ci_tokens table
ALTER TABLE ci_tokens ADD COLUMN IF NOT EXISTS created_by UUID REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE ci_tokens ADD COLUMN IF NOT EXISTS last_used_at TIMESTAMPTZ;
ALTER TABLE ci_tokens ADD COLUMN IF NOT EXISTS revoked_by UUID REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE ci_tokens ADD COLUMN IF NOT EXISTS description TEXT;

-- Add unique constraint on project_id + lowercase name
CREATE UNIQUE INDEX IF NOT EXISTS ci_tokens_project_name_idx
    ON ci_tokens (project_id, lower(name));

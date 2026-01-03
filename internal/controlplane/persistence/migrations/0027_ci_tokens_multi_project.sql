-- Migration: CI Tokens Multi-Project Support
-- Transforms ci_tokens from per-project to org-level with multi-project scopes

-- Step 1: Add organization_id column
ALTER TABLE ci_tokens ADD COLUMN IF NOT EXISTS organization_id UUID;

-- Step 2: Backfill organization_id from projects table
UPDATE ci_tokens t
SET organization_id = p.organization_id
FROM projects p
WHERE t.project_id = p.id AND t.organization_id IS NULL;

-- Step 3: Make organization_id NOT NULL (after backfill)
ALTER TABLE ci_tokens ALTER COLUMN organization_id SET NOT NULL;

-- Step 4: Add foreign key to organizations
ALTER TABLE ci_tokens ADD CONSTRAINT ci_tokens_org_fk
    FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE;

-- Step 5: Create junction table for token-project relationships
CREATE TABLE IF NOT EXISTS ci_token_projects (
    token_id UUID NOT NULL REFERENCES ci_tokens(id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    scope TEXT NOT NULL CHECK(scope IN ('read', 'write')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (token_id, project_id)
);

-- Index for efficient project-based lookups
CREATE INDEX IF NOT EXISTS ci_token_projects_project_idx ON ci_token_projects(project_id);

-- Step 6: Backfill ci_token_projects from existing ci_tokens
-- For each existing token, create a row in the junction table
-- If scopes contains 'write' -> write, otherwise read (default to write for CI tokens)
INSERT INTO ci_token_projects (token_id, project_id, scope, created_at)
SELECT
    ct.id,
    ct.project_id,
    CASE
        WHEN 'write' = ANY(ct.scopes) THEN 'write'
        WHEN ct.scopes IS NULL OR array_length(ct.scopes, 1) IS NULL THEN 'write'
        ELSE 'read'
    END,
    ct.created_at
FROM ci_tokens ct
WHERE ct.project_id IS NOT NULL
ON CONFLICT (token_id, project_id) DO NOTHING;

-- Step 7: Drop old indexes that reference project_id
DROP INDEX IF EXISTS ci_tokens_project_idx;
DROP INDEX IF EXISTS ci_tokens_project_name_idx;

-- Step 8: Drop old columns
ALTER TABLE ci_tokens DROP COLUMN IF EXISTS project_id;
ALTER TABLE ci_tokens DROP COLUMN IF EXISTS scopes;

-- Step 9: Add new uniqueness constraint on name within org
CREATE UNIQUE INDEX IF NOT EXISTS ci_tokens_org_name_idx ON ci_tokens (organization_id, lower(name));

-- Migration: Add source_ref to projects, suites, and tests to support multi-branch discovery
-- This allows the same suite/test name to exist on different branches without unique constraint conflicts

-- Step 1: Add source_ref column to projects
ALTER TABLE projects ADD COLUMN IF NOT EXISTS source_ref TEXT;

-- Backfill existing projects: use default_branch as source_ref
UPDATE projects SET source_ref = default_branch WHERE source_ref IS NULL;

-- Make source_ref NOT NULL after backfill
ALTER TABLE projects ALTER COLUMN source_ref SET NOT NULL;

-- Ensure path_scope is never null (safety backfill)
UPDATE projects SET path_scope = '[]'::jsonb WHERE path_scope IS NULL;

-- Step 2: Change project uniqueness to include source_ref
-- Drop the old unique index
DROP INDEX IF EXISTS projects_org_name_idx;

-- Create new unique index including source_ref
CREATE UNIQUE INDEX IF NOT EXISTS projects_org_name_ref_idx
    ON projects (organization_id, lower(name), lower(source_ref));

-- Step 3: Add source_ref column to suites
ALTER TABLE suites ADD COLUMN IF NOT EXISTS source_ref TEXT;

-- Backfill suites: try to get ref from config->>'ref', else fall back to parent project's default_branch
UPDATE suites s
SET source_ref = COALESCE(NULLIF(s.config->>'ref', ''), p.default_branch)
FROM projects p
WHERE p.id = s.project_id AND s.source_ref IS NULL;

-- Make source_ref NOT NULL after backfill
ALTER TABLE suites ALTER COLUMN source_ref SET NOT NULL;

-- Step 4: Change suite uniqueness to include source_ref
-- Drop the old unique index
DROP INDEX IF EXISTS suites_project_name_idx;

-- Create new unique index including source_ref
CREATE UNIQUE INDEX IF NOT EXISTS suites_project_name_ref_idx
    ON suites (project_id, lower(name), lower(source_ref));

-- Step 5: Add source_ref column to tests
ALTER TABLE tests ADD COLUMN IF NOT EXISTS source_ref TEXT;

-- Backfill tests from parent suite's source_ref
UPDATE tests t
SET source_ref = s.source_ref
FROM suites s
WHERE s.id = t.suite_id AND t.source_ref IS NULL;

-- Make source_ref NOT NULL after backfill
ALTER TABLE tests ALTER COLUMN source_ref SET NOT NULL;

-- Step 6: Change test uniqueness to include source_ref
-- Drop the old unique index
DROP INDEX IF EXISTS tests_suite_name_idx;

-- Create new unique index including source_ref
CREATE UNIQUE INDEX IF NOT EXISTS tests_suite_name_ref_idx
    ON tests (suite_id, lower(name), lower(source_ref));

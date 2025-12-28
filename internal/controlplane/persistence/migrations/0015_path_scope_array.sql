-- Migration: Ensure path_scope is always a JSON array (never null or jsonb 'null')
--
-- Bug fix: projects.path_scope can be stored as jsonb literal null even though the column is NOT NULL.
-- Migration 0014 only does WHERE path_scope IS NULL which doesn't catch jsonb null.

-- Step 1: Backfill jsonb literal null to empty array
UPDATE projects SET path_scope = '[]'::jsonb WHERE path_scope = 'null'::jsonb;

-- Step 2: Backfill SQL NULL (defensive, shouldn't exist but be safe)
UPDATE projects SET path_scope = '[]'::jsonb WHERE path_scope IS NULL;

-- Step 3: Add constraint to enforce array type
-- Use DO block since PostgreSQL lacks IF NOT EXISTS for constraints
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'projects_path_scope_is_array'
    ) THEN
        ALTER TABLE projects ADD CONSTRAINT projects_path_scope_is_array
            CHECK (jsonb_typeof(path_scope) = 'array');
    END IF;
END $$;

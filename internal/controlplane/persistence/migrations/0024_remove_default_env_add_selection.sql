-- Remove "default environment" concept and replace with per-user environment selection
-- This migration:
-- 1. Drops the unique index that enforces one default per project
-- 2. Drops the is_default column from project_environments
-- 3. Creates a new table for per-user, per-project environment selection

-- Drop the default environment uniqueness index
DROP INDEX IF EXISTS project_environments_default_idx;

-- Drop the is_default column
ALTER TABLE project_environments DROP COLUMN IF EXISTS is_default;

-- Create the per-user environment selection table
CREATE TABLE IF NOT EXISTS project_environment_selections (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    environment_id UUID NOT NULL REFERENCES project_environments(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, project_id)
);

-- Index for looking up all selections for a project (useful for cleanup)
CREATE INDEX IF NOT EXISTS project_environment_selections_project_idx
    ON project_environment_selections (project_id);

-- Index for looking up selections by environment (useful for cascade logic)
CREATE INDEX IF NOT EXISTS project_environment_selections_env_idx
    ON project_environment_selections (environment_id);

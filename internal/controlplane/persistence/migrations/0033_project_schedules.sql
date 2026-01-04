-- Create project_schedules table for project-level scheduled runs
-- Each schedule is environment-scoped: at most 1 schedule per (project, environment)
CREATE TABLE IF NOT EXISTS project_schedules (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    environment_id UUID NOT NULL REFERENCES project_environments(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    cron_expression TEXT NOT NULL,
    timezone TEXT NOT NULL DEFAULT 'UTC',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    next_run_at TIMESTAMPTZ,
    last_run_at TIMESTAMPTZ,
    last_run_id TEXT,
    last_run_status TEXT,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Unique constraint: at most 1 schedule per (project, environment)
CREATE UNIQUE INDEX IF NOT EXISTS project_schedules_project_env_idx
    ON project_schedules (project_id, environment_id);

-- Index for scheduler polling: find enabled schedules due to run
CREATE INDEX IF NOT EXISTS project_schedules_due_idx
    ON project_schedules (next_run_at ASC)
    WHERE enabled = TRUE AND next_run_at IS NOT NULL;

-- Index for listing schedules by project
CREATE INDEX IF NOT EXISTS project_schedules_project_idx
    ON project_schedules (project_id);

-- Add schedule_type column to runs to distinguish project vs suite schedules
ALTER TABLE runs ADD COLUMN IF NOT EXISTS schedule_type TEXT;

-- Drop the existing FK constraint on schedule_id that references suite_schedules
-- Since schedule_id can now reference either project_schedules or suite_schedules,
-- we can't use a FK constraint and rely on application-level validation + schedule_type
ALTER TABLE runs DROP CONSTRAINT IF EXISTS runs_schedule_id_fkey;

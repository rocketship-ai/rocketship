-- Create suite_schedules table for scheduled test runs
CREATE TABLE IF NOT EXISTS suite_schedules (
    id UUID PRIMARY KEY,
    suite_id UUID NOT NULL REFERENCES suites(id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT,
    cron_expression TEXT NOT NULL,
    timezone TEXT NOT NULL DEFAULT 'UTC',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    environment_id UUID REFERENCES project_environments(id) ON DELETE SET NULL,
    next_run_at TIMESTAMPTZ,
    last_run_at TIMESTAMPTZ,
    last_run_id TEXT,
    last_run_status TEXT,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Each suite can only have one schedule with the same name
CREATE UNIQUE INDEX IF NOT EXISTS suite_schedules_suite_name_idx
    ON suite_schedules (suite_id, lower(name));

-- Index for listing schedules by suite
CREATE INDEX IF NOT EXISTS suite_schedules_suite_idx
    ON suite_schedules (suite_id);

-- Index for listing schedules by project
CREATE INDEX IF NOT EXISTS suite_schedules_project_idx
    ON suite_schedules (project_id);

-- Index for finding next scheduled runs (for scheduler polling)
CREATE INDEX IF NOT EXISTS suite_schedules_next_run_idx
    ON suite_schedules (next_run_at ASC)
    WHERE enabled = TRUE AND next_run_at IS NOT NULL;

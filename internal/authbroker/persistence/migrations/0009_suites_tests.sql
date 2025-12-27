-- Create suites table for storing test suite definitions
CREATE TABLE IF NOT EXISTS suites (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT,
    file_path TEXT,
    config JSONB NOT NULL DEFAULT '{}'::jsonb,
    test_count INTEGER NOT NULL DEFAULT 0,
    last_run_id TEXT,
    last_run_status TEXT,
    last_run_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Each project can only have one suite with the same name
CREATE UNIQUE INDEX IF NOT EXISTS suites_project_name_idx
    ON suites (project_id, lower(name));

-- Index for listing suites by project
CREATE INDEX IF NOT EXISTS suites_project_idx
    ON suites (project_id);

-- Index for recent activity queries
CREATE INDEX IF NOT EXISTS suites_last_run_at_idx
    ON suites (last_run_at DESC)
    WHERE last_run_at IS NOT NULL;

-- Create tests table for storing individual test definitions
CREATE TABLE IF NOT EXISTS tests (
    id UUID PRIMARY KEY,
    suite_id UUID NOT NULL REFERENCES suites(id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT,
    config JSONB NOT NULL DEFAULT '{}'::jsonb,
    step_count INTEGER NOT NULL DEFAULT 0,
    last_run_id TEXT,
    last_run_status TEXT,
    last_run_at TIMESTAMPTZ,
    pass_rate NUMERIC(5,2),
    avg_duration_ms INTEGER,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Each suite can only have one test with the same name
CREATE UNIQUE INDEX IF NOT EXISTS tests_suite_name_idx
    ON tests (suite_id, lower(name));

-- Index for listing tests by suite
CREATE INDEX IF NOT EXISTS tests_suite_idx
    ON tests (suite_id);

-- Index for listing tests by project (for cross-suite queries)
CREATE INDEX IF NOT EXISTS tests_project_idx
    ON tests (project_id);

-- Index for test health queries (pass rate ordering)
CREATE INDEX IF NOT EXISTS tests_pass_rate_idx
    ON tests (project_id, pass_rate ASC NULLS LAST);

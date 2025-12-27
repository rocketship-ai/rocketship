-- Add additional columns to runs table
ALTER TABLE runs ADD COLUMN IF NOT EXISTS environment_id UUID REFERENCES project_environments(id) ON DELETE SET NULL;
ALTER TABLE runs ADD COLUMN IF NOT EXISTS schedule_id UUID REFERENCES suite_schedules(id) ON DELETE SET NULL;
ALTER TABLE runs ADD COLUMN IF NOT EXISTS commit_message TEXT;
ALTER TABLE runs ADD COLUMN IF NOT EXISTS skipped_tests INTEGER NOT NULL DEFAULT 0;

-- Create run_tests table for individual test results within a run
CREATE TABLE IF NOT EXISTS run_tests (
    id UUID PRIMARY KEY,
    run_id TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    test_id UUID REFERENCES tests(id) ON DELETE SET NULL,
    workflow_id TEXT NOT NULL,
    name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'PENDING',
    error_message TEXT,
    started_at TIMESTAMPTZ,
    ended_at TIMESTAMPTZ,
    duration_ms INTEGER,
    step_count INTEGER NOT NULL DEFAULT 0,
    passed_steps INTEGER NOT NULL DEFAULT 0,
    failed_steps INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for listing tests by run
CREATE INDEX IF NOT EXISTS run_tests_run_idx
    ON run_tests (run_id);

-- Index for looking up by workflow_id (for workflow monitoring)
CREATE UNIQUE INDEX IF NOT EXISTS run_tests_workflow_idx
    ON run_tests (workflow_id);

-- Create run_steps table for individual step results within a test run
CREATE TABLE IF NOT EXISTS run_steps (
    id UUID PRIMARY KEY,
    run_test_id UUID NOT NULL REFERENCES run_tests(id) ON DELETE CASCADE,
    step_index INTEGER NOT NULL,
    name TEXT NOT NULL,
    plugin TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'PENDING',
    error_message TEXT,
    request_data JSONB,
    response_data JSONB,
    assertions_passed INTEGER NOT NULL DEFAULT 0,
    assertions_failed INTEGER NOT NULL DEFAULT 0,
    started_at TIMESTAMPTZ,
    ended_at TIMESTAMPTZ,
    duration_ms INTEGER,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for listing steps by run_test
CREATE INDEX IF NOT EXISTS run_steps_run_test_idx
    ON run_steps (run_test_id);

-- Index for ordering steps within a test
CREATE INDEX IF NOT EXISTS run_steps_order_idx
    ON run_steps (run_test_id, step_index);

-- Create run_logs table for storing execution logs
CREATE TABLE IF NOT EXISTS run_logs (
    id UUID PRIMARY KEY,
    run_id TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    run_test_id UUID REFERENCES run_tests(id) ON DELETE CASCADE,
    run_step_id UUID REFERENCES run_steps(id) ON DELETE CASCADE,
    level TEXT NOT NULL DEFAULT 'INFO',
    message TEXT NOT NULL,
    metadata JSONB,
    logged_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for listing logs by run (time-ordered)
CREATE INDEX IF NOT EXISTS run_logs_run_idx
    ON run_logs (run_id, logged_at ASC);

-- Index for listing logs by test (for test-level log viewing)
CREATE INDEX IF NOT EXISTS run_logs_run_test_idx
    ON run_logs (run_test_id, logged_at ASC)
    WHERE run_test_id IS NOT NULL;

-- Create run_artifacts table for storing test artifacts (screenshots, files, etc.)
CREATE TABLE IF NOT EXISTS run_artifacts (
    id UUID PRIMARY KEY,
    run_id TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    run_test_id UUID REFERENCES run_tests(id) ON DELETE CASCADE,
    run_step_id UUID REFERENCES run_steps(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    artifact_type TEXT NOT NULL,
    mime_type TEXT,
    size_bytes BIGINT,
    storage_path TEXT NOT NULL,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for listing artifacts by run
CREATE INDEX IF NOT EXISTS run_artifacts_run_idx
    ON run_artifacts (run_id);

-- Index for listing artifacts by test
CREATE INDEX IF NOT EXISTS run_artifacts_run_test_idx
    ON run_artifacts (run_test_id)
    WHERE run_test_id IS NOT NULL;

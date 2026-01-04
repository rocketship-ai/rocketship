-- Indexes for Test Health page scalability

-- Speed "latest schedule results per test" lateral join
-- This index supports efficient lookups of recent run_tests for a specific test_id,
-- sorted by created_at descending (for getting last N results).
CREATE INDEX IF NOT EXISTS run_tests_test_id_created_at_idx
ON run_tests (test_id, created_at DESC)
WHERE test_id IS NOT NULL;

-- Speed scheduled-run filtering by trigger/env/time
-- This partial index covers only scheduled runs (trigger = 'schedule'),
-- supporting efficient queries for environment-scoped schedule history.
CREATE INDEX IF NOT EXISTS runs_trigger_env_created_at_idx
ON runs (trigger, environment_id, created_at DESC)
WHERE trigger = 'schedule';

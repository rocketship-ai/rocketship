-- Drop indexes
DROP INDEX IF EXISTS idx_test_runs_started_at;
DROP INDEX IF EXISTS idx_test_runs_suite_name;
DROP INDEX IF EXISTS idx_test_runs_status;
DROP INDEX IF EXISTS idx_test_runs_triggered_by;
DROP INDEX IF EXISTS idx_test_runs_repository_id;

-- Drop table
DROP TABLE IF EXISTS test_runs;
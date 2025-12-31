-- Add unique index on run_steps for upsert operations
-- This allows us to upsert steps by (run_test_id, step_index)
CREATE UNIQUE INDEX IF NOT EXISTS run_steps_run_test_step_idx ON run_steps (run_test_id, step_index);

-- Add unique constraint for suite schedules: at most 1 schedule per (suite, environment)
-- This enables environment-scoped overrides where each suite can have a custom schedule per environment.

-- Create unique index for (suite_id, environment_id) where environment_id is not null
-- This enforces: at most one override schedule per suite per environment
CREATE UNIQUE INDEX IF NOT EXISTS suite_schedules_suite_env_idx
    ON suite_schedules (suite_id, environment_id)
    WHERE environment_id IS NOT NULL;

-- Add joined fields for environment lookup in schedule queries
-- Note: The SuiteSchedule type already has environment_id, we just need to ensure
-- the queries join with project_environments to get name/slug.

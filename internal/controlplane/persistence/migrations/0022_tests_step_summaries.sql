-- Add step_summaries JSONB column to tests table
-- This stores step definitions (plugin, name) extracted from YAML at scan time

-- Add the column with empty array default
ALTER TABLE tests
    ADD COLUMN IF NOT EXISTS step_summaries JSONB NOT NULL DEFAULT '[]'::jsonb;

-- Backfill any NULL values to empty array (defensive)
UPDATE tests SET step_summaries = '[]'::jsonb WHERE step_summaries IS NULL;

-- Add constraint to ensure it's always an array
ALTER TABLE tests
    ADD CONSTRAINT tests_step_summaries_is_array
    CHECK (jsonb_typeof(step_summaries) = 'array');

-- Create an index for potential future queries on step_summaries
CREATE INDEX IF NOT EXISTS tests_step_summaries_idx
    ON tests USING gin (step_summaries);

-- Add extended data columns to run_steps for rich step details UI
-- These columns store per-step assertion results, saved variables, and step configuration

-- Assertion results: array of {type, path, expected, actual, passed, message}
ALTER TABLE run_steps ADD COLUMN IF NOT EXISTS assertions_data JSONB;

-- Saved variables: array of {name, value, source_type, source}
ALTER TABLE run_steps ADD COLUMN IF NOT EXISTS variables_data JSONB;

-- Step configuration snapshot (sanitized step definition)
ALTER TABLE run_steps ADD COLUMN IF NOT EXISTS step_config JSONB;

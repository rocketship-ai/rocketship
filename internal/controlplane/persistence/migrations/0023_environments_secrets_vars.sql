-- Extend project_environments to support separate env_secrets and config_vars
-- env_secrets: flat map of secret key-value pairs (referenced via {{ .env.* }})
-- config_vars: nested JSON object of config variables (referenced via {{ .vars.* }})

-- Add new columns
ALTER TABLE project_environments
    ADD COLUMN IF NOT EXISTS env_secrets JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE project_environments
    ADD COLUMN IF NOT EXISTS config_vars JSONB NOT NULL DEFAULT '{}'::jsonb;

-- Backfill: copy existing 'variables' column data into 'env_secrets'
-- (treat the old flat map as secrets for v1 backwards compatibility)
UPDATE project_environments
SET env_secrets = COALESCE(variables, '{}'::jsonb)
WHERE variables IS NOT NULL AND variables != '{}'::jsonb;

-- Drop the old 'variables' column
ALTER TABLE project_environments
    DROP COLUMN IF EXISTS variables;

-- Add CHECK constraints to ensure both are JSON objects (not arrays)
ALTER TABLE project_environments
    ADD CONSTRAINT project_environments_env_secrets_is_object
    CHECK (jsonb_typeof(env_secrets) = 'object');

ALTER TABLE project_environments
    ADD CONSTRAINT project_environments_config_vars_is_object
    CHECK (jsonb_typeof(config_vars) = 'object');

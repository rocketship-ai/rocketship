-- Add yaml_payload column to suites for storing raw YAML content at scan time
-- This enables scheduled runs to use the YAML as it existed when scanned (Git-as-SoT)
ALTER TABLE suites ADD COLUMN IF NOT EXISTS yaml_payload TEXT NOT NULL DEFAULT '';

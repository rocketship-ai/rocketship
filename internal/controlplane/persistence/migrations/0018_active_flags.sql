-- Migration: Add is_active flags to projects, suites, and tests
-- This allows PR-branch discovery rows to be hidden after merge while preserving run history

-- Add is_active column to projects
ALTER TABLE projects ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS deactivated_at TIMESTAMPTZ;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS deactivated_reason TEXT;

-- Add is_active column to suites
ALTER TABLE suites ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE suites ADD COLUMN IF NOT EXISTS deactivated_at TIMESTAMPTZ;
ALTER TABLE suites ADD COLUMN IF NOT EXISTS deactivated_reason TEXT;

-- Add is_active column to tests
ALTER TABLE tests ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE tests ADD COLUMN IF NOT EXISTS deactivated_at TIMESTAMPTZ;
ALTER TABLE tests ADD COLUMN IF NOT EXISTS deactivated_reason TEXT;

-- Add indexes for efficient filtering by is_active
CREATE INDEX IF NOT EXISTS projects_org_active_idx ON projects (organization_id, is_active);
CREATE INDEX IF NOT EXISTS suites_project_active_idx ON suites (project_id, is_active);
CREATE INDEX IF NOT EXISTS tests_suite_active_idx ON tests (suite_id, is_active);

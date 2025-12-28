-- Migration 0012: GitHub App installations
-- Tracks which GitHub App installations are linked to organizations.
-- This replaces the OAuth repo scope approach for per-repo access control.

-- Drop old OAuth token table if present (cleanup from prior implementation)
DROP TABLE IF EXISTS github_oauth_tokens;

-- GitHub App installations table
CREATE TABLE IF NOT EXISTS github_app_installations (
    organization_id UUID PRIMARY KEY REFERENCES organizations(id) ON DELETE CASCADE,
    installation_id BIGINT NOT NULL UNIQUE,
    installed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    account_login TEXT,
    account_type TEXT,  -- 'User' or 'Organization'
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for looking up by installation_id (used in callback)
CREATE INDEX IF NOT EXISTS idx_github_app_installations_installation_id
    ON github_app_installations(installation_id);

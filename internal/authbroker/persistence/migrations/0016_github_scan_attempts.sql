-- Migration: Add github_scan_attempts table to track webhook-driven scans
-- This provides a verifiable audit trail for repository scans per organization

CREATE TABLE IF NOT EXISTS github_scan_attempts (
    id UUID PRIMARY KEY,
    delivery_id TEXT NOT NULL REFERENCES github_webhook_deliveries(delivery_id) ON DELETE CASCADE,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    repository_full_name TEXT NOT NULL,
    source_ref TEXT NOT NULL,
    head_sha TEXT,
    status TEXT NOT NULL CHECK (status IN ('success', 'error', 'skipped')),
    error_message TEXT,
    suites_found INT NOT NULL DEFAULT 0,
    tests_found INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for querying scan attempts by organization
CREATE INDEX IF NOT EXISTS github_scan_attempts_org_idx ON github_scan_attempts (organization_id);

-- Index for querying scan attempts by delivery (for debugging webhook processing)
CREATE INDEX IF NOT EXISTS github_scan_attempts_delivery_idx ON github_scan_attempts (delivery_id);

-- Index for querying recent scan attempts
CREATE INDEX IF NOT EXISTS github_scan_attempts_created_idx ON github_scan_attempts (created_at DESC);

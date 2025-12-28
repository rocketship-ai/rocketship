-- Migration 0013: GitHub webhook deliveries
-- Tracks received webhook deliveries for audit and verification purposes.

CREATE TABLE IF NOT EXISTS github_webhook_deliveries (
    delivery_id TEXT PRIMARY KEY,
    event TEXT NOT NULL,
    repository_full_name TEXT,
    ref TEXT,
    action TEXT,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for querying recent events by type
CREATE INDEX IF NOT EXISTS idx_github_webhook_deliveries_event_received
    ON github_webhook_deliveries(event, received_at DESC);

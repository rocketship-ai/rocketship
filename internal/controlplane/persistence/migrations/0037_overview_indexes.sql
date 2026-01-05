-- Add indexes for overview query performance
-- These indexes improve the performance of aggregate queries on the runs table

-- Index for overview queries filtering by org and ended_at (24h window queries)
CREATE INDEX IF NOT EXISTS runs_org_ended_at_idx
    ON runs (organization_id, ended_at DESC)
    WHERE ended_at IS NOT NULL;

-- Index for overview queries filtering by org and status (in-progress runs)
CREATE INDEX IF NOT EXISTS runs_org_status_idx
    ON runs (organization_id, status)
    WHERE status IN ('RUNNING', 'PENDING');

-- Index for overview time-series queries (pass rate over time)
CREATE INDEX IF NOT EXISTS runs_org_project_ended_at_idx
    ON runs (organization_id, project_id, ended_at DESC)
    WHERE ended_at IS NOT NULL AND started_at IS NOT NULL;

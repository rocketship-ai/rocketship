-- Index to support efficient querying of runs for suite activity
-- Used by GET /api/suites/{suiteId}/runs endpoint
CREATE INDEX IF NOT EXISTS runs_org_project_suite_created_idx
    ON runs (organization_id, project_id, suite_name, created_at DESC);

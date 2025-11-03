CREATE TABLE IF NOT EXISTS runs (
    id TEXT PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    project_id UUID REFERENCES projects(id) ON DELETE SET NULL,
    status TEXT NOT NULL,
    suite_name TEXT NOT NULL,
    initiator TEXT NOT NULL,
    trigger TEXT NOT NULL DEFAULT '',
    schedule_name TEXT NOT NULL DEFAULT '',
    config_source TEXT NOT NULL,
    source TEXT NOT NULL DEFAULT '',
    branch TEXT NOT NULL DEFAULT '',
    environment TEXT NOT NULL DEFAULT '',
    commit_sha TEXT,
    bundle_sha TEXT,
    total_tests INTEGER NOT NULL DEFAULT 0,
    passed_tests INTEGER NOT NULL DEFAULT 0,
    failed_tests INTEGER NOT NULL DEFAULT 0,
    timeout_tests INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    ended_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS runs_org_created_idx ON runs (organization_id, created_at DESC);

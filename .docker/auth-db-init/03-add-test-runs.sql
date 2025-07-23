-- Test Runs (enhanced with auth info)
CREATE TABLE test_runs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    suite_name VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL,
    repository_id UUID REFERENCES repositories(id),
    file_path VARCHAR(500) NOT NULL, -- path to rocketship.yaml
    branch VARCHAR(255),
    commit_sha VARCHAR(40),
    triggered_by VARCHAR(255) REFERENCES users(id),
    authorized_teams UUID[], -- teams that authorized this run
    metadata JSONB,
    started_at TIMESTAMPTZ DEFAULT NOW(),
    ended_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Indexes for performance
CREATE INDEX idx_test_runs_repository_id ON test_runs(repository_id);
CREATE INDEX idx_test_runs_triggered_by ON test_runs(triggered_by);
CREATE INDEX idx_test_runs_status ON test_runs(status);
CREATE INDEX idx_test_runs_suite_name ON test_runs(suite_name);
CREATE INDEX idx_test_runs_started_at ON test_runs(started_at);
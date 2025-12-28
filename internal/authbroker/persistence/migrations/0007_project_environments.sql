-- Create project_environments table for managing deployment environments per project
CREATE TABLE IF NOT EXISTS project_environments (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    slug TEXT NOT NULL,
    description TEXT,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    variables JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Each project can only have one environment with the same slug
CREATE UNIQUE INDEX IF NOT EXISTS project_environments_project_slug_idx
    ON project_environments (project_id, lower(slug));

-- Index for listing environments by project
CREATE INDEX IF NOT EXISTS project_environments_project_idx
    ON project_environments (project_id);

-- Ensure only one default environment per project
CREATE UNIQUE INDEX IF NOT EXISTS project_environments_default_idx
    ON project_environments (project_id)
    WHERE is_default = TRUE;

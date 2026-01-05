-- Add index for suite runs branch pagination queries
-- Optimizes queries that filter by org/project/suite/branch and order by created_at
CREATE INDEX IF NOT EXISTS runs_org_project_suite_branch_created_idx
ON runs (organization_id, project_id, suite_name, branch, created_at DESC);

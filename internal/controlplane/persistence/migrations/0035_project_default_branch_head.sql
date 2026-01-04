-- Add columns to track the default branch HEAD commit for scheduled runs
-- The scheduler reads these values to populate commit_sha and commit_message on scheduled runs
-- The scanner writes these values when scanning the default branch

ALTER TABLE projects ADD COLUMN IF NOT EXISTS default_branch_head_sha TEXT;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS default_branch_head_message TEXT;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS default_branch_head_at TIMESTAMPTZ;

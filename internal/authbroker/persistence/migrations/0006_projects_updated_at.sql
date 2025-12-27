-- Add updated_at and last_synced_at to projects table
ALTER TABLE projects ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
ALTER TABLE projects ADD COLUMN IF NOT EXISTS last_synced_at TIMESTAMPTZ;

-- Backfill updated_at with created_at for existing rows
UPDATE projects SET updated_at = created_at;

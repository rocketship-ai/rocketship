-- Migration: Add suite_file_path column to runs table for file-path-based suite linking
-- This allows runs to be associated with suites by file path (stable identity) rather than
-- suite name (which can change).

-- Step 1: Add the suite_file_path column (nullable for backwards compatibility)
ALTER TABLE runs ADD COLUMN IF NOT EXISTS suite_file_path TEXT;

-- Step 2: Create index for efficient suite run listing queries
-- This index supports the common query pattern: list runs for a suite by file path
CREATE INDEX IF NOT EXISTS runs_suite_file_path_idx
    ON runs (organization_id, project_id, lower(suite_file_path), created_at DESC)
    WHERE suite_file_path IS NOT NULL;

-- Step 3: Backfill suite_file_path from existing run data
-- For runs that have linked tests (via run_tests.test_id), we can resolve the suite's file_path
-- through the chain: runs → run_tests → tests → suites
WITH backfill_data AS (
    SELECT DISTINCT ON (r.id)
        r.id as run_id,
        s.file_path as suite_file_path
    FROM runs r
    JOIN run_tests rt ON rt.run_id = r.id
    JOIN tests t ON t.id = rt.test_id
    JOIN suites s ON s.id = t.suite_id
    WHERE r.suite_file_path IS NULL
      AND rt.test_id IS NOT NULL
      AND s.file_path IS NOT NULL
    ORDER BY r.id, rt.created_at ASC
)
UPDATE runs r
SET suite_file_path = bd.suite_file_path
FROM backfill_data bd
WHERE r.id = bd.run_id;

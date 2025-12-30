-- Migration: Enforce suite identity constraints
-- Suite identity = (project_id, source_ref, file_path) with additional name uniqueness

-- Step 1: Delete tests belonging to suites with invalid file_path
-- (tests FK has ON DELETE CASCADE, but we're explicit for clarity)
DELETE FROM tests WHERE suite_id IN (
    SELECT id FROM suites WHERE file_path IS NULL OR file_path = ''
);

-- Step 2: Delete suites with invalid file_path (legacy/bad rows)
DELETE FROM suites WHERE file_path IS NULL OR file_path = '';

-- Step 3: Deduplicate by (project_id, source_ref, file_path), keeping newest updated_at
WITH duplicates AS (
    SELECT id, ROW_NUMBER() OVER (
        PARTITION BY project_id, lower(source_ref), lower(file_path)
        ORDER BY updated_at DESC
    ) AS rn
    FROM suites
)
DELETE FROM tests WHERE suite_id IN (SELECT id FROM duplicates WHERE rn > 1);

DELETE FROM suites WHERE id IN (
    SELECT id FROM (
        SELECT id, ROW_NUMBER() OVER (
            PARTITION BY project_id, lower(source_ref), lower(file_path)
            ORDER BY updated_at DESC
        ) AS rn
        FROM suites
    ) d WHERE d.rn > 1
);

-- Step 4: Deduplicate by (project_id, source_ref, name), keeping newest updated_at
WITH duplicates AS (
    SELECT id, ROW_NUMBER() OVER (
        PARTITION BY project_id, lower(source_ref), lower(name)
        ORDER BY updated_at DESC
    ) AS rn
    FROM suites
)
DELETE FROM tests WHERE suite_id IN (SELECT id FROM duplicates WHERE rn > 1);

DELETE FROM suites WHERE id IN (
    SELECT id FROM (
        SELECT id, ROW_NUMBER() OVER (
            PARTITION BY project_id, lower(source_ref), lower(name)
            ORDER BY updated_at DESC
        ) AS rn
        FROM suites
    ) d WHERE d.rn > 1
);

-- Step 5: Enforce NOT NULL on file_path
ALTER TABLE suites ALTER COLUMN file_path SET NOT NULL;

-- Step 6: Create unique indexes with exact names for isUniqueViolation() matching
-- Primary identity: file_path
CREATE UNIQUE INDEX IF NOT EXISTS suites_project_file_ref_idx
    ON suites (project_id, lower(file_path), lower(source_ref));

-- Name uniqueness across files within same project/ref
CREATE UNIQUE INDEX IF NOT EXISTS suites_project_name_ref_idx
    ON suites (project_id, lower(name), lower(source_ref));

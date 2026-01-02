-- Standardize runs.trigger / runs.source values
-- No backwards compatibility - normalize all legacy values

-- Step 1: Normalize trigger values
-- 'webhook' → 'ci' (webhook was an old name for CI-triggered runs)
UPDATE runs SET trigger = 'ci' WHERE trigger = 'webhook';

-- Empty triggers from manual runs should be 'manual'
UPDATE runs SET trigger = 'manual' WHERE trigger = '' AND source IN ('cli-local', 'web-ui', '');

-- Empty triggers from CI sources should be 'ci'
UPDATE runs SET trigger = 'ci' WHERE trigger = '' AND source IN ('ci-branch', 'ci', 'github-actions', 'ci-token');

-- Any remaining empty triggers default to 'manual'
UPDATE runs SET trigger = 'manual' WHERE trigger = '';

-- Step 2: Normalize source values
-- 'ci-branch' → 'github-actions' (clearer provenance)
UPDATE runs SET source = 'github-actions' WHERE source = 'ci-branch';

-- 'ci' → 'ci-token' (since CI token runs set source='ci')
-- Only do this if the initiator indicates a CI token
UPDATE runs SET source = 'ci-token' WHERE source = 'ci' AND initiator LIKE 'ci_token:%';

-- For other 'ci' sources without ci_token initiator, assume github-actions
UPDATE runs SET source = 'github-actions' WHERE source = 'ci';

-- Step 3: Normalize initiator format for manual runs
-- Format: user:<github_username>
-- Only update if initiator doesn't already have a prefix and is not empty/unknown
UPDATE runs SET initiator = 'user:' || initiator
WHERE trigger = 'manual'
  AND initiator != ''
  AND initiator != 'unknown'
  AND initiator NOT LIKE 'user:%'
  AND initiator NOT LIKE 'ci_token:%'
  AND initiator NOT LIKE 'schedule:%';

-- Step 4: Add CHECK constraint to enforce trigger enum
-- Only allow: 'manual', 'ci', 'schedule'
ALTER TABLE runs ADD CONSTRAINT runs_trigger_check
CHECK (trigger IN ('manual', 'ci', 'schedule'));

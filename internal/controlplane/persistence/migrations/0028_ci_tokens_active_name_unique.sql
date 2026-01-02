-- Allow reusing CI token names after revocation
-- Only enforce name uniqueness among non-revoked (active) tokens

-- Drop the old index that enforced uniqueness across all tokens
DROP INDEX IF EXISTS ci_tokens_org_name_idx;

-- Create partial unique index that only applies to non-revoked tokens
-- This allows users to reuse a token name once the previous token is revoked
CREATE UNIQUE INDEX IF NOT EXISTS ci_tokens_org_active_name_idx
ON ci_tokens (organization_id, lower(name))
WHERE revoked_at IS NULL;

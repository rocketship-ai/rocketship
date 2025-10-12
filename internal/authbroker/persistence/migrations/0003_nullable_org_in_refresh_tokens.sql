-- Allow refresh tokens for pending users (users without organizations yet)
ALTER TABLE refresh_tokens ALTER COLUMN organization_id DROP NOT NULL;

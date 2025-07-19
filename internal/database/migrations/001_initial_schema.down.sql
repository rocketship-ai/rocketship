-- Drop indexes
DROP INDEX IF EXISTS idx_teams_name;
DROP INDEX IF EXISTS idx_users_email;
DROP INDEX IF EXISTS idx_api_tokens_hash;
DROP INDEX IF EXISTS idx_api_tokens_team_id;
DROP INDEX IF EXISTS idx_repositories_url;
DROP INDEX IF EXISTS idx_team_members_user_id;

-- Drop tables (in reverse order of creation)
DROP TABLE IF EXISTS api_tokens;
DROP TABLE IF EXISTS team_repositories;
DROP TABLE IF EXISTS repositories;
DROP TABLE IF EXISTS team_members;
DROP TABLE IF EXISTS teams;
DROP TABLE IF EXISTS users;

-- Drop extension
DROP EXTENSION IF EXISTS "uuid-ossp";
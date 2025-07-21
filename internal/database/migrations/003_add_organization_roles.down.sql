-- Revert organization roles back to boolean is_admin
ALTER TABLE users 
ADD COLUMN is_admin BOOLEAN DEFAULT false;

-- Migrate data back: org_admin becomes true, org_member becomes false
UPDATE users SET is_admin = true WHERE org_role = 'org_admin';
UPDATE users SET is_admin = false WHERE org_role = 'org_member';

-- Remove the org_role column
ALTER TABLE users DROP COLUMN org_role;
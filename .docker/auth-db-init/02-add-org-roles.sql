-- Update users table to use organization roles instead of boolean is_admin
ALTER TABLE users 
ADD COLUMN org_role VARCHAR(50) DEFAULT 'org_member' 
CHECK (org_role IN ('org_admin', 'org_member'));

-- Migrate existing data: admin users become org_admin, others become org_member
UPDATE users SET org_role = 'org_admin' WHERE is_admin = true;
UPDATE users SET org_role = 'org_member' WHERE is_admin = false;

-- Remove the old is_admin column
ALTER TABLE users DROP COLUMN is_admin;
-- Remove unused description column from project_environments table
ALTER TABLE project_environments DROP COLUMN IF EXISTS description;

-- Rename organization_admins to organization_owners
-- This aligns with our terminology: "owner" for org-level, "admin" reserved for future project-level use

ALTER TABLE organization_admins RENAME TO organization_owners;

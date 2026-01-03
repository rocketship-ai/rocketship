-- Project invites for email-based invitation flow
CREATE TABLE project_invites (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    email text NOT NULL,
    invited_by uuid NOT NULL REFERENCES users(id),
    code_hash bytea NOT NULL,
    code_salt bytea NOT NULL,
    expires_at timestamptz NOT NULL,
    accepted_at timestamptz,
    accepted_by uuid REFERENCES users(id),
    revoked_at timestamptz,
    revoked_by uuid REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- Index for looking up invites by org and email
CREATE INDEX idx_project_invites_org_email ON project_invites(organization_id, lower(email));

-- Index for expiry cleanup
CREATE INDEX idx_project_invites_expires_at ON project_invites(expires_at);

-- Join table: which projects each invite grants access to
CREATE TABLE project_invite_projects (
    invite_id uuid NOT NULL REFERENCES project_invites(id) ON DELETE CASCADE,
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    role text NOT NULL CHECK (role IN ('read', 'write')),
    PRIMARY KEY (invite_id, project_id)
);

-- Index for looking up invites by project
CREATE INDEX idx_project_invite_projects_project ON project_invite_projects(project_id);

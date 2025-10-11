CREATE TABLE IF NOT EXISTS organization_registrations (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    org_name TEXT NOT NULL,
    code_hash BYTEA NOT NULL,
    code_salt BYTEA NOT NULL,
    attempts INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 10,
    expires_at TIMESTAMPTZ NOT NULL,
    resend_available_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS organization_registrations_user_idx ON organization_registrations (user_id);
CREATE INDEX IF NOT EXISTS organization_registrations_expires_idx ON organization_registrations (expires_at);

CREATE TABLE IF NOT EXISTS organization_invites (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('admin')),
    code_hash BYTEA NOT NULL,
    code_salt BYTEA NOT NULL,
    invited_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    accepted_at TIMESTAMPTZ,
    accepted_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS organization_invites_org_idx ON organization_invites (organization_id);
CREATE INDEX IF NOT EXISTS organization_invites_email_idx ON organization_invites (LOWER(email));
CREATE INDEX IF NOT EXISTS organization_invites_expires_idx ON organization_invites (expires_at);

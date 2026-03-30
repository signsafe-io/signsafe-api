CREATE TABLE pending_invitations (
    id              VARCHAR(26)  PRIMARY KEY,
    organization_id VARCHAR(26)  NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    invited_by      VARCHAR(26)  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email           VARCHAR(255) NOT NULL,
    role            VARCHAR(50)  NOT NULL DEFAULT 'member',
    token           VARCHAR(64)  NOT NULL UNIQUE,
    expires_at      TIMESTAMPTZ  NOT NULL,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (organization_id, email)
);

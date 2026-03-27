-- 000004_init_audit.up.sql
-- Audit Events (INSERT only, enforced by trigger)

CREATE TABLE audit_events (
    id              VARCHAR(26)  PRIMARY KEY,
    actor_id        VARCHAR(26)  REFERENCES users(id),
    actor_email     VARCHAR(255),
    action          VARCHAR(100) NOT NULL,
    -- action types: SIGNUP, LOGIN, LOGOUT, UPLOAD_CONTRACT, VIEW_CONTRACT,
    --               REQUEST_ANALYSIS, VIEW_EVIDENCE, OVERRIDE_RISK, etc.
    target_type     VARCHAR(100),
    target_id       VARCHAR(26),
    organization_id VARCHAR(26)  REFERENCES organizations(id),
    context         JSONB        NOT NULL DEFAULT '{}',
    ip_address      INET,
    user_agent      TEXT,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_events_actor_id ON audit_events(actor_id);
CREATE INDEX idx_audit_events_action ON audit_events(action);
CREATE INDEX idx_audit_events_target ON audit_events(target_type, target_id);
CREATE INDEX idx_audit_events_organization_id ON audit_events(organization_id);
CREATE INDEX idx_audit_events_created_at ON audit_events(created_at DESC);

-- Prevent UPDATE and DELETE on audit_events (immutable audit log)
CREATE OR REPLACE FUNCTION prevent_audit_events_modification()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'audit_events is immutable: UPDATE and DELETE are not allowed';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_audit_events_no_update
    BEFORE UPDATE ON audit_events
    FOR EACH ROW EXECUTE FUNCTION prevent_audit_events_modification();

CREATE TRIGGER trg_audit_events_no_delete
    BEFORE DELETE ON audit_events
    FOR EACH ROW EXECUTE FUNCTION prevent_audit_events_modification();

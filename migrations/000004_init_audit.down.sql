-- 000004_init_audit.down.sql
DROP TRIGGER IF EXISTS trg_audit_events_no_delete ON audit_events;
DROP TRIGGER IF EXISTS trg_audit_events_no_update ON audit_events;
DROP FUNCTION IF EXISTS prevent_audit_events_modification();
DROP TABLE IF EXISTS audit_events;

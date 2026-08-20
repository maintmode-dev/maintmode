-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';
-- ============================================================
-- Audit log table.
-- Stores ALL significant events: authentication, RBAC changes,
-- resource CRUD, access denials.
--
-- IMPORTANT: there are NO foreign keys to other tables.
-- entity_id and target_id are TEXT, not UUID FKs.
-- That allows us to:
--   1. Keep the audit trail after the entity is deleted
--   2. Avoid blocking deletion because of FK constraints
--   3. Support different kinds of ID
-- ============================================================
CREATE TABLE IF NOT EXISTS audit_log (
    id          UUID        PRIMARY KEY DEFAULT uuidv7(),
    action      TEXT        NOT NULL,               -- event type: login_success, role_assigned, resource_created, access_denied, ...
    actor       TEXT        NOT NULL,               -- who performed the action (id, email, etc)
    entity_id   TEXT        NOT NULL DEFAULT '',    -- ID of the primary entity (user_id, etc)
    entity_type TEXT        NOT NULL DEFAULT '',    -- type of the primary entity: user, article, role, policy
    target_id   TEXT        NOT NULL DEFAULT '',    -- ID of the secondary entity (optional)
    target_type TEXT        NOT NULL DEFAULT '',    -- type of the secondary entity (optional)
    details     TEXT        NOT NULL DEFAULT '',    -- human-readable description
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for the typical queries:
--   "all events for user alice"
CREATE INDEX IF NOT EXISTS idx_audit_log_entity ON audit_log (entity_type, entity_id);
--   "all access_denied over the last hour"
CREATE INDEX IF NOT EXISTS idx_audit_log_action ON audit_log (action);
--   "all actions by admin today"
CREATE INDEX IF NOT EXISTS idx_audit_log_actor ON audit_log (actor);
--   "chronology and pagination"
CREATE INDEX IF NOT EXISTS idx_audit_log_created_at ON audit_log (created_at DESC);
--   "all events where user alice is the target of the action"
CREATE INDEX IF NOT EXISTS idx_audit_log_target ON audit_log (target_type, target_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
DROP TABLE IF EXISTS audit_log;
-- +goose StatementEnd

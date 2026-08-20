-- +goose Up
-- +goose StatementBegin
-- RUK-171: structured audit trail for AuditLogPage.
--   actor_id           — stable actor ID (user UUID); empty for system/unidentified.
--                        NOT an FK — the audit trail outlives user deletion.
--   actor_display_name — snapshot of the actor name at event time (point-in-time,
--                        not resolved on read: audit must not change retroactively).
--   metadata           — whitelist structure of action-specific fields (ip, user_agent,
--                        session_id, failure_reason, roles, ...). No tokens/secrets.
ALTER TABLE audit_log
    ADD COLUMN IF NOT EXISTS actor_id           TEXT  NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS actor_display_name TEXT  NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS metadata           JSONB NOT NULL DEFAULT '{}'::jsonb;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE audit_log
    DROP COLUMN IF EXISTS actor_id,
    DROP COLUMN IF EXISTS actor_display_name,
    DROP COLUMN IF EXISTS metadata;
-- +goose StatementEnd

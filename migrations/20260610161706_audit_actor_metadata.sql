-- +goose Up
-- +goose StatementBegin
-- RUK-171: структурированный аудит для AuditLogPage.
--   actor_id           — стабильный ID актора (user UUID); пустой для system/неопознанных.
--                        НЕ FK — аудит переживает удаление пользователя.
--   actor_display_name — снапшот имени актора на момент события (point-in-time,
--                        не резолвится на чтении: аудит не должен меняться задним числом).
--   metadata           — whitelist-структура action-specific полей (ip, user_agent,
--                        session_id, failure_reason, roles, ...). Без токенов/секретов.
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

-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';
-- ============================================================
-- Таблица аудит-лога
-- Хранит ВСЕ значимые события: аутентификацию, RBAC-изменения,
-- CRUD ресурсов, отказы доступа.
--
-- ВАЖНО: НЕТ foreign keys на другие таблицы.
-- entity_id и target_id — TEXT, не UUID FK.
-- Это позволяет:
--   1. Сохранять аудит после удаления сущности
--   2. Не блокировать удаление из-за FK constraints
--   3. Поддерживать разные типы ID
-- ============================================================
CREATE TABLE IF NOT EXISTS audit_log (
    id          UUID        PRIMARY KEY DEFAULT uuidv7(),
    action      TEXT        NOT NULL,               -- тип события: login_success, role_assigned, resource_created, access_denied, ...
    actor       TEXT        NOT NULL,               -- кто совершил действие (id, email, etc)
    entity_id   TEXT        NOT NULL DEFAULT '',    -- ID основной сущности (user_id, etc)
    entity_type TEXT        NOT NULL DEFAULT '',    -- тип основной сущности: user, article, role, policy
    target_id   TEXT        NOT NULL DEFAULT '',    -- ID второй сущности (опционально)
    target_type TEXT        NOT NULL DEFAULT '',    -- тип второй сущности (опционально)
    details     TEXT        NOT NULL DEFAULT '',    -- человекочитаемое описание
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Индексы для типовых запросов:
--   "все события для user alice"
CREATE INDEX IF NOT EXISTS idx_audit_log_entity ON audit_log (entity_type, entity_id);
--   "все access_denied за последний час"
CREATE INDEX IF NOT EXISTS idx_audit_log_action ON audit_log (action);
--   "все действия admin за сегодня"
CREATE INDEX IF NOT EXISTS idx_audit_log_actor ON audit_log (actor);
--   "хронология и пагинация"
CREATE INDEX IF NOT EXISTS idx_audit_log_created_at ON audit_log (created_at DESC);
--   "все события, где user alice — цель действия"
CREATE INDEX IF NOT EXISTS idx_audit_log_target ON audit_log (target_type, target_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
DROP TABLE IF EXISTS audit_log;
-- +goose StatementEnd

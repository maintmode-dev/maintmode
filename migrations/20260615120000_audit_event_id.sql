-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';
-- ============================================================
-- RUK-179: idempotency key for outbox-delivered audit events.
--
-- Audit events are now delivered through the goque transactional
-- outbox and written by the drain processor. event_id is a per-event
-- UUID minted at publish time and carried in the task payload. The
-- UNIQUE constraint makes the write idempotent: a redelivered task
-- (INSERT succeeded but the ack was lost) collapses to a no-op via
-- ON CONFLICT (event_id) DO NOTHING in the store.
--
-- Nullable + no DEFAULT: pre-RUK-179 rows have no event_id. A plain
-- (non-partial) UNIQUE index treats NULLs as distinct, so legacy rows
-- coexist freely, while non-null event_ids are unique. The index is
-- deliberately NOT partial: ON CONFLICT (event_id) DO NOTHING only
-- infers a non-partial index, so a partial one would break the
-- idempotent upsert in the store.
-- ============================================================
ALTER TABLE audit_log
    ADD COLUMN IF NOT EXISTS event_id UUID;

CREATE UNIQUE INDEX IF NOT EXISTS idx_audit_log_event_id
    ON audit_log (event_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
DROP INDEX IF EXISTS idx_audit_log_event_id;
ALTER TABLE audit_log DROP COLUMN IF EXISTS event_id;
-- +goose StatementEnd

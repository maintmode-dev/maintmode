-- +goose Up
-- +goose StatementBegin
-- goque >= v0.12 fences every worker write on a per-task version, so a worker
-- that lost its task cannot overwrite the new owner's result. Mirrors goque's
-- own migrations/pg/20261016120000_task_version.sql; existing rows start at 0
-- on the metadata-only default. Must be applied before the new binary rolls
-- out: the old binary ignores the column, the new one cannot run without it.
ALTER TABLE goque_task
    ADD COLUMN version BIGINT NOT NULL DEFAULT 0;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE goque_task
    DROP COLUMN version;
-- +goose StatementEnd

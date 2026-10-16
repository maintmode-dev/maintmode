-- +goose Up
-- +goose StatementBegin
-- An optional link to the maintenance's task in an external tracker. Empty
-- means "no link"; existing rows get it on the metadata-only default.
ALTER TABLE maintenances
    ADD COLUMN task_tracker_url TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Drops every stored link.
ALTER TABLE maintenances
    DROP COLUMN task_tracker_url;
-- +goose StatementEnd

-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';
alter table audit_log drop column target_id;
alter table audit_log drop column target_type;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
-- +goose StatementEnd

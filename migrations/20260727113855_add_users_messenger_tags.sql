-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';

-- telegram_tag / slack_tag: messenger handles of the user, stored exactly as
-- entered (no normalization, a leading "@" is kept if the user typed one). NULL
-- means "not provided" — a notification degrades to the user's display name.
ALTER TABLE users ADD COLUMN telegram_tag TEXT;
ALTER TABLE users ADD COLUMN slack_tag TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
ALTER TABLE users DROP COLUMN telegram_tag;
ALTER TABLE users DROP COLUMN slack_tag;
-- +goose StatementEnd

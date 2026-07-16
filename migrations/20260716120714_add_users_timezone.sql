-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';

-- timezone: the user's preferred IANA timezone (e.g. "Asia/Nicosia"). NULL means
-- "not selected" — the frontend falls back to browser auto-detect. Stored as an
-- IANA string (never a numeric offset) so it stays correct across DST.
ALTER TABLE users ADD COLUMN timezone TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
ALTER TABLE users DROP COLUMN timezone;
-- +goose StatementEnd

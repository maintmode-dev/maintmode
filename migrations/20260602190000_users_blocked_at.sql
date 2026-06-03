-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';

-- blocked_at: NULL means the user is active. A non-null timestamp records when an
-- admin blocked the user. Roles are preserved on block so unblock restores access.
ALTER TABLE users ADD COLUMN blocked_at TIMESTAMPTZ;
CREATE INDEX idx_users_blocked_at ON users (blocked_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
DROP INDEX IF EXISTS idx_users_blocked_at;
ALTER TABLE users DROP COLUMN blocked_at;
-- +goose StatementEnd

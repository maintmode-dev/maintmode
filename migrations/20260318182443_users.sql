-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';
CREATE TABLE users (
    id                  UUID PRIMARY KEY DEFAULT uuidv7(),
    email               TEXT NOT NULL UNIQUE,
    name                TEXT NOT NULL DEFAULT '',
    oauth_provider_id   TEXT NOT NULL UNIQUE,
    roles               TEXT[] NOT NULL DEFAULT '{}',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_users_oauth_provider_id ON users (oauth_provider_id);
CREATE INDEX idx_users_email ON users (email);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
DROP TABLE IF EXISTS users;
-- +goose StatementEnd

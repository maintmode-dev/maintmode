-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';
CREATE TABLE refresh_tokens (
    token_hash  TEXT PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    family      UUID NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    grace_ttl   TIMESTAMPTZ,
    revoked     BOOLEAN NOT NULL DEFAULT FALSE,
    replaced_by TEXT,
    bound_ip    TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ
);

CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens (user_id);
CREATE INDEX idx_refresh_tokens_family ON refresh_tokens (family);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
DROP TABLE IF EXISTS refresh_tokens;
-- +goose StatementEnd

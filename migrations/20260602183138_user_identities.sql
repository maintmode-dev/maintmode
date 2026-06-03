-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';

-- user_identities normalizes the user <-> OAuth provider link into a 1:N
-- relation so a single user can sign in via multiple providers. It replaces
-- the denormalized users.oauth_provider_id column (one provider per user).
-- The service is not yet in production, so no data migration is needed —
-- schema change only.
CREATE TABLE user_identities (
    id          UUID        PRIMARY KEY DEFAULT uuidv7(),
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider    TEXT        NOT NULL, -- OAuth provider id (google, github, ...)
    subject     TEXT        NOT NULL, -- provider's stable user id (OIDC "sub" claim); unique per provider
    email       TEXT        NOT NULL DEFAULT '', -- email reported by the provider at link time
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON COLUMN user_identities.subject IS
    'Stable per-user identifier issued by the OAuth provider (the OIDC "sub" claim from the id_token). Identifies who the user is at that provider; used to resolve the user on login.';

-- A given provider subject identifies exactly one user across the system.
CREATE UNIQUE INDEX user_identities_provider_subject_uidx ON user_identities (provider, subject);
-- A user has at most one identity per provider. This keeps the row count per
-- user equal to the number of connected providers, which the disconnect
-- lockout guard and connected_providers both rely on.
CREATE UNIQUE INDEX user_identities_user_provider_uidx ON user_identities (user_id, provider);
CREATE INDEX idx_user_identities_user_id ON user_identities (user_id);

DROP INDEX IF EXISTS idx_users_oauth_provider_id;
ALTER TABLE users DROP COLUMN oauth_provider_id;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';

-- NOTE: this rollback assumes an empty (or fresh) users table — the service is
-- not yet in production. Re-adding oauth_provider_id as NOT NULL UNIQUE has no
-- DEFAULT, so on a populated users table the ADD COLUMN would fail. If users
-- ever holds data when rolling back, backfill the column before this step.
DROP TABLE IF EXISTS user_identities;

ALTER TABLE users ADD COLUMN oauth_provider_id TEXT NOT NULL UNIQUE;
CREATE INDEX idx_users_oauth_provider_id ON users (oauth_provider_id);
-- +goose StatementEnd

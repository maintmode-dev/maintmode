-- +goose Up
-- +goose StatementBegin
-- Break-glass was the only method that wrote an identity with no registry row
-- behind it (builtin_method = 'bootstrap'). It now signs in to an account found
-- by its reserved address and writes no identity at all, so every identity is a
-- registry-backed provider's: the built-in branch of the table goes, and
-- integration_id becomes required in place of the CHECK that paired the two.
--
-- The rows it leaves are break-glass identities of the previous design; they
-- reach nothing any more.
DELETE FROM user_identities WHERE builtin_method IS NOT NULL;

DROP INDEX user_identities_builtin_subject_uidx;
DROP INDEX user_identities_user_builtin_uidx;

ALTER TABLE user_identities
    DROP CONSTRAINT user_identities_provider_target_chk,
    DROP COLUMN builtin_method,
    ALTER COLUMN integration_id SET NOT NULL;

COMMENT ON COLUMN user_identities.integration_id IS
    'The integration_settings row (category login) this identity authenticates against.';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Restores the shape, not the deleted break-glass identities: the previous
-- binary recreates its own on the next break-glass sign-in.
ALTER TABLE user_identities
    ALTER COLUMN integration_id DROP NOT NULL,
    ADD COLUMN builtin_method TEXT;

ALTER TABLE user_identities
    ADD CONSTRAINT user_identities_provider_target_chk
        CHECK (num_nonnulls(integration_id, builtin_method) = 1);

CREATE UNIQUE INDEX user_identities_builtin_subject_uidx
    ON user_identities (builtin_method, subject) WHERE builtin_method IS NOT NULL;
CREATE UNIQUE INDEX user_identities_user_builtin_uidx
    ON user_identities (user_id, builtin_method) WHERE builtin_method IS NOT NULL;

COMMENT ON COLUMN user_identities.integration_id IS
    'The integration_settings row (category login) this identity authenticates against. NULL for a built-in method, which has no registry row -- see builtin_method.';
-- +goose StatementEnd

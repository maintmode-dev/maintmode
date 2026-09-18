-- +goose Up
-- +goose StatementBegin
-- auth_settings answers ONE question per built-in sign-in method: should the
-- login page offer it, and should the backend accept it. Nothing else.
--
-- A separate table rather than a row in integration_settings, and the reason is
-- the registry's own shape: every row there carries config, secrets and a
-- NOT NULL dek_id referencing data_keys, because a registry row describes a
-- connection to an EXTERNAL system. email_otp and email_password have no
-- endpoint, no credentials and nothing to encrypt -- a row for them would mean
-- minting a DEK to protect an empty secrets object, and registering a fake
-- integration implementation so Registry.admit had something to resolve.
--
-- Login PROVIDERS are not in here. They keep integration_settings.enabled,
-- which already works and which the reloader already enforces by building a
-- disabled provider with neither method nor gateway. Two switches over one
-- provider would mean every read had to answer which of them wins.
--
-- bootstrap is deliberately absent. It is the break-glass credential: always
-- enabled, never listed on the public endpoint, and a row here would be a
-- switch that must never be thrown.
CREATE TABLE auth_settings (
    id                 UUID        PRIMARY KEY DEFAULT uuidv7(),
    method             TEXT        NOT NULL,
    enabled            BOOLEAN     NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by_user_id UUID,
    UNIQUE (method)
);

COMMENT ON COLUMN auth_settings.method IS
    'Built-in sign-in method: email_otp | email_password. No CHECK, matching integration_settings: the set of supported methods is a fact about the code, not the schema, and the service validates it against a closed set.';
COMMENT ON COLUMN auth_settings.enabled IS
    'Whether the method is offered on the login page AND accepted at sign-in. NOT NULL with no default on purpose: a row must state its answer, and a DEFAULT true would let a future INSERT that forgets the column silently open a sign-in path.';
COMMENT ON COLUMN auth_settings.updated_by_user_id IS
    'Authorship captured from the access token; no FK to users (owned by auth, resolved on read), mirroring integration_settings / messenger_channels / resources.';

-- Seeded rather than treating a missing row as "enabled". Fail-open on a
-- missing row would mean any future bug that loses one silently reopens a
-- sign-in path an admin closed, and it would make the table's contents depend
-- on whether anyone had ever opened the screen. Seeding makes every read a
-- straight lookup and every missing row unambiguously a fault.
--
-- email_password ON, email_otp OFF. The default is the narrower of the two
-- sign-in surfaces an instance could start with: a code mailed to an address
-- turns the mail channel into a credential, so every instance that has SMTP
-- configured would otherwise ship with a second way in that nobody asked for.
-- An admin who wants it turns it on deliberately, which is the whole point of
-- the table.
--
-- email_password is the one that must stay ON, because it is the only built-in
-- an instance is guaranteed to be able to use: it needs nothing configured.
-- Seeding both OFF would lock out every instance that has no provider yet,
-- which is all of them at the moment this migration runs.
--
-- This is NOT observably neutral, unlike the rest of this migration: an
-- existing instance whose users sign in with email codes loses that path the
-- moment the new binary starts reading this table. The migration is still safe
-- to apply BEFORE that binary -- the old one ignores the table entirely -- but
-- the CUTOVER is the breaking moment, not the migration. An instance that wants
-- to keep email codes must flip the row back on, and the deploy note has to say
-- so. The reverse default was rejected for the reason above: shipping a method
-- on and having admins discover it is worse than shipping it off and having
-- them ask for it.
INSERT INTO auth_settings (method, enabled) VALUES
    ('email_otp', FALSE),
    ('email_password', TRUE);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Three things to know before running this, because each is a way to make an
-- outage worse while trying to recover from one.
--
-- 1. Roll the BINARY back first, and wait until no replica is still running the
--    new one -- then this. On a rolling rollback those are different
--    instructions: dropping the table while any new replica still serves makes
--    that replica refuse every ordinary password sign-in until it is replaced.
--    The old binary ignores this table entirely, so once it is running the drop
--    is invisible.
--
-- 2. This RE-OPENS every built-in method, and that is wider than undoing the
--    admin's own changes: email_otp ships OFF, so dropping the table restores
--    an instance to accepting email codes it may never have accepted. An
--    instance deliberately running SSO-only silently starts accepting passwords
--    and codes again. That is a security regression, not just data loss.
--
-- 3. The disabled state survives only in the logs and the audit trail, and in
--    the scenario that prompts a rollback -- nobody can sign in -- the audit
--    trail is also unreachable, being asynchronous and admin-gated. The line an
--    operator can actually read is the "auth method toggled" Info in the
--    container log. Disables must be re-applied by hand after rolling forward.
--
-- Point 2 argues for keeping the table instead of dropping it: the old binary
-- ignores it, so a preserved table would be a no-op backward and lossless
-- forward. It is dropped anyway, because a Down that does not undo its Up
-- misleads the next operator reading `goose down`, and the audit trail already
-- holds the state. No guard on a non-empty table, unlike integration_settings:
-- there are no secrets and no user data here, only two flags.
DROP TABLE auth_settings;
-- +goose StatementEnd

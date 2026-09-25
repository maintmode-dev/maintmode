-- +goose Up
-- +goose StatementBegin
-- user_identities.provider held a provider's system name as free text, with no
-- foreign key. The relation between an identity and the registry row that
-- vouches for it was defended entirely in Go, and an orphaned row was not inert:
-- it waited for the same name to be created again against a DIFFERENT IdP, and
-- then handed that IdP an account predating it.
--
-- The column becomes a reference. What it cannot reference -- the break-glass
-- admin and the dev stub, neither of which has a registry row -- moves to a
-- second column, with a CHECK making the two mutually exclusive.
--
-- Scope, stated because the code comments this migration replaces overclaimed:
-- this closes the ORPHAN. Re-pointing a live row at another IdP, and re-claiming
-- an account by email after a delete, are account-level paths a foreign key has
-- nothing to say about. They remain open.

-- 1. The rows go. They carry a name that no longer has a column to live in, and
--    there is nothing to map it to: the registry row a dev-stand identity names
--    may never have existed. Affordable only because the feature is unreleased
--    -- the same reason 20260911233949 deleted login rows outright.
--
--    This MUST precede the CHECK below: a surviving row has both new columns
--    NULL, which num_nonnulls() = 1 rejects, and the ALTER would fail on any
--    stand where somebody has signed in.
DELETE FROM user_identities;

ALTER TABLE user_identities
    -- ON DELETE RESTRICT, not CASCADE. The cascade stays in Go
    -- (services/integration.Service.Delete), where deleting a provider's
    -- accounts is visible at the call site and a test can observe it going
    -- missing. A schema-level CASCADE would keep working silently if that code
    -- were ever removed, which is precisely what makes it the wrong tool here.
    --
    -- RESTRICT's audience is the programmer, not the admin: Delete clears the
    -- identities BEFORE removing the row, so an ordinary delete still answers
    -- 204 whether the provider carried one account or a hundred thousand.
    ADD COLUMN integration_id UUID REFERENCES integration_settings (id) ON DELETE RESTRICT,
    ADD COLUMN builtin_method TEXT;

COMMENT ON COLUMN user_identities.integration_id IS
    'The integration_settings row (category login) this identity authenticates against. NULL for a built-in method, which has no registry row -- see builtin_method.';
COMMENT ON COLUMN user_identities.builtin_method IS
    'The built-in sign-in method this identity belongs to, for methods with no registry row. NULL for a registry-backed provider. Exactly one of the two columns is set.';

-- 2. Exactly one target. A row with neither is unattributable; a row with both
--    would let the two branches disagree about who vouched for the account.
ALTER TABLE user_identities
    ADD CONSTRAINT user_identities_provider_target_chk
        CHECK (num_nonnulls(integration_id, builtin_method) = 1);

-- No CHECK enum on builtin_method, matching integration_settings.kind and
-- entity.AuthMethodName: which sign-in methods exist is a fact about the code,
-- and a second copy of that list here would be a second place to change --
-- discovered, when the two disagree, as a write the database refuses and the
-- code believes valid.
--
-- entity.AuthMethod.IsBuiltin owns the set. The column holds whatever that
-- vocabulary admits.

-- 3. Uniqueness, split per branch -- and this is the trap the change had to
--    avoid. Under a nullable column the old full indexes keep EXISTING while
--    silently ceasing to hold: in PostgreSQL NULL is never equal to NULL, so
--    rows with integration_id IS NULL never conflict with each other. Two
--    identical break-glass identities would have become insertable, the index
--    would still be listed, and every test would still have passed.
DROP INDEX user_identities_provider_subject_uidx;
DROP INDEX user_identities_user_provider_uidx;

-- A given provider subject identifies exactly one user across the system.
CREATE UNIQUE INDEX user_identities_integration_subject_uidx
    ON user_identities (integration_id, subject) WHERE integration_id IS NOT NULL;
CREATE UNIQUE INDEX user_identities_builtin_subject_uidx
    ON user_identities (builtin_method, subject) WHERE builtin_method IS NOT NULL;

-- A user has at most one identity per method. The disconnect lockout guard
-- counts rows per user and relies on this to be exact.
CREATE UNIQUE INDEX user_identities_user_integration_uidx
    ON user_identities (user_id, integration_id) WHERE integration_id IS NOT NULL;
CREATE UNIQUE INDEX user_identities_user_builtin_uidx
    ON user_identities (user_id, builtin_method) WHERE builtin_method IS NOT NULL;

-- idx_user_identities_user_id is deliberately untouched: CountByUserID and
-- ListProvidersByUserIDs read it, and neither is affected by this change.

-- 4. The column this migration exists to remove.
ALTER TABLE user_identities DROP COLUMN provider;

-- 5. integration_settings.name explained its own immutability by the orphaning
--    this migration just made impossible. The immutability SURVIVES -- the
--    secret AAD is sealed under the name, so renaming would break decryption --
--    but the reason had to change, or the schema would document a threat that
--    no longer exists.
COMMENT ON COLUMN integration_settings.name IS
    'System the row connects to -- slack, telegram, email, google, custom -- and the registry key that decides which implementation parses it. Immutable: it is an input to the AAD of this row''s client_secret, so renaming would make the secret undecryptable. Linked accounts are NOT at risk -- user_identities references this row by id.';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Refuses on a non-empty table rather than guessing, for the same reason
-- 20260911233949 does: restoring provider TEXT NOT NULL needs a name per row,
-- and a row whose registry parent has since been deleted has nothing to map
-- back to. A Down that "succeeds" while rolling nothing back leaves the schema
-- new and goose_db_version old, which is the worst available outcome.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM user_identities LIMIT 1) THEN
        RAISE EXCEPTION 'user_identities is not empty: integration_id cannot be mapped back to a provider name'
            USING HINT = 'A registry-backed row could be resolved through integration_settings only while its parent still exists, and a built-in row has no registry name at all. Empty the table first if that loss is intended.';
    END IF;
END
$$;

-- Empty table: reverse in full. Half a rollback is as bad as none.
DROP INDEX user_identities_integration_subject_uidx;
DROP INDEX user_identities_builtin_subject_uidx;
DROP INDEX user_identities_user_integration_uidx;
DROP INDEX user_identities_user_builtin_uidx;

ALTER TABLE user_identities
    DROP CONSTRAINT user_identities_provider_target_chk,
    DROP COLUMN integration_id,
    DROP COLUMN builtin_method;

ALTER TABLE user_identities ADD COLUMN provider TEXT NOT NULL;

CREATE UNIQUE INDEX user_identities_provider_subject_uidx ON user_identities (provider, subject);
CREATE UNIQUE INDEX user_identities_user_provider_uidx ON user_identities (user_id, provider);

COMMENT ON COLUMN integration_settings.name IS
    'System the row connects to -- slack, telegram, email, google, custom -- and the registry key that decides which implementation parses it. Immutable: for a login provider it is written verbatim into user_identities.provider, so renaming orphans every linked account.';
-- +goose StatementEnd

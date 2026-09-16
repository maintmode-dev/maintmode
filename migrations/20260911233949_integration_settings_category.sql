-- +goose Up
-- +goose StatementBegin
-- kind stops naming the system and starts naming the CATEGORY -- notify or
-- login -- while the system moves into name. The placeholder goes with it:
-- 'default' existed only because the column could not be empty once kind was
-- already spoken for.
--
-- A SECOND file rather than an edit to 20260910232233. goose keys applied
-- migrations by version_id alone, with no checksum, so on a stand where that
-- version is already recorded an edited body would never run -- the column
-- default, the old kinds and the litter would all survive. Editing it would fix
-- only databases that have never seen it.

-- Part A: the transport CHECK. Safe only because NotifyTransport.IsValid() has
-- already stopped accepting email (see the commit that did it): the constraint
-- was the single thing standing between an admin and a notify channel that
-- mails an arbitrary external address, and the Go guard is now that thing.
--
-- The constraint is declared inline and unnamed on the creating table, so
-- messenger_channels_transport_check is a name PostgreSQL DERIVED. IF EXISTS
-- would let a stand where it was derived differently drop nothing and succeed
-- silently, leaving the Down to fail later on a duplicate object -- so the drop
-- is asserted instead.
ALTER TABLE messenger_channels DROP CONSTRAINT IF EXISTS messenger_channels_transport_check;

DO $$
BEGIN
    -- Narrowed to constraints that mention transport. Asserting "no CHECK at
    -- all" would fail this migration on an unrelated one added later -- a
    -- length check on transport_channel_id, say -- with a message pointing at
    -- the wrong thing.
    IF EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conrelid = 'messenger_channels'::regclass
           AND contype = 'c'
           AND pg_get_constraintdef(oid) LIKE '%transport %'
    ) THEN
        RAISE EXCEPTION 'a CHECK constraint on messenger_channels.transport survived the drop'
            USING HINT = 'It exists under a name this migration does not know. Drop it by its real name, then re-run.';
    END IF;
END
$$;

-- Part B: the category rewrite.
ALTER TABLE integration_settings ALTER COLUMN name DROP DEFAULT;

-- 1. Litter from API runs: uuid-suffixed kinds no registry ever registered, so
--    nothing can read them. Left behind they would hold values that are neither
--    a category nor a system.
DELETE FROM integration_settings
 WHERE kind NOT IN ('slack', 'telegram', 'email', 'oidc', 'github_oauth');

-- 2. Stored login providers. DESTRUCTIVE, and the only such statement here.
--
--    The registry's key moved from the kind to the system name, and that key is
--    an input to SecretAADForClient -- so a secret sealed as 'oidc' cannot be
--    opened as 'google' or 'custom'. Carrying these rows forward would leave
--    providers that decrypt nowhere and fail at the next sign-in.
--
--    Deleting them is affordable only because the feature is unreleased: login
--    rows exist solely on stands where someone typed one in during development.
--    An operator re-enters the client secret through the admin UI.
--
--    github_oauth is included because the kind itself is removed: GitHub has no
--    discovery document, so its AAD issuer is empty by construction and a
--    preset protects nothing there. GitHub sign-in returns as its own task.
DELETE FROM integration_settings
 WHERE kind IN ('oidc', 'github_oauth');

-- 3. The rewrite. The two login branches are kept and are correct but inert
--    after step 2 -- they cover a row that arrived between deploy steps.
UPDATE integration_settings SET name = kind, kind = 'notify'
 WHERE kind IN ('slack', 'telegram', 'email');

UPDATE integration_settings SET name = 'custom', kind = 'login'
 WHERE kind = 'oidc';

COMMENT ON COLUMN integration_settings.kind IS
    'Category the row belongs to: ''notify'' (delivery) or ''login'' (sign-in provider).';
COMMENT ON COLUMN integration_settings.name IS
    'System the row connects to -- slack, telegram, email, google, custom -- and the registry key that decides which implementation parses it. Immutable: for a login provider it is written verbatim into user_identities.provider, so renaming orphans every linked account.';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Refuses on a non-empty table rather than guessing. A notify row's old kind is
-- recoverable from its name, but a login row has no pre-migration form -- the
-- one it had was deleted by Up precisely because it could not be carried
-- forward -- so there is nothing to map it back to.
--
-- A Down that "succeeds" while rolling nothing back is the worst available
-- outcome: the schema ends up new and goose_db_version old. So this one either
-- refuses loudly or actually reverses.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM integration_settings LIMIT 1) THEN
        RAISE EXCEPTION 'integration_settings is not empty: kind/name cannot be mapped back'
            USING HINT = 'A notify row is recoverable from its name, but a login row has no pre-migration form. Empty the table first if that loss is intended.';
    END IF;

    IF EXISTS (SELECT 1 FROM messenger_channels WHERE transport NOT IN ('slack', 'telegram')) THEN
        RAISE EXCEPTION 'messenger_channels holds a transport the restored CHECK would reject'
            USING HINT = 'Remove those channels first.';
    END IF;
END
$$;
-- +goose StatementEnd

-- Past the guard the table is empty and messenger_channels holds only the two
-- transports the CHECK permits, so these restore the schema and there is no
-- data to map. (An earlier draft also replayed the category UPDATEs backwards;
-- they are unreachable here, and were not an inverse anyway -- Up maps oidc to
-- custom, so replaying it would restore the new name rather than the old kind.)
ALTER TABLE messenger_channels
    ADD CONSTRAINT messenger_channels_transport_check CHECK (transport IN ('slack', 'telegram'));

ALTER TABLE integration_settings ALTER COLUMN name SET DEFAULT 'default';

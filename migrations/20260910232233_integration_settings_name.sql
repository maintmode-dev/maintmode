-- +goose Up
-- +goose StatementBegin
-- Multi-connection, which the original table comment called "an additive future
-- change": login providers are inherently plural (Google alongside a corporate
-- Keycloak), so the identity of a row becomes (kind, name) rather than kind.
--
-- name defaults to 'default' and the default STAYS on the column. It is not
-- tidy-up to remove later: Create inserts MutableColumns, so a binary that
-- predates this column keeps inserting successfully and the default supplies
-- the value. That is what makes a binary rollback survive at all -- though only
-- until a second row exists for some kind, after which the old code's
-- kind-only lookup reads an arbitrary row.
--
-- The empty string is deliberately NOT the backfill value: it travels through a
-- URL path silently and fails loudest in production. Single-instance kinds
-- (slack, telegram, email) keep 'default' forever and never think about it.
ALTER TABLE integration_settings
    ADD COLUMN name TEXT NOT NULL DEFAULT 'default';

COMMENT ON COLUMN integration_settings.name IS
    'Instance name within a kind. For login providers it is the identity users authenticate against and is written verbatim into user_identities.provider, so it is immutable: renaming orphans every linked account. Single-instance kinds carry ''default''.';

-- integration_settings_kind_key is the name PostgreSQL derived for the unnamed
-- UNIQUE (kind) in the creating migration. IF EXISTS covers a stand where it
-- was created under another name rather than failing mid-deploy.
ALTER TABLE integration_settings
    DROP CONSTRAINT IF EXISTS integration_settings_kind_key;

ALTER TABLE integration_settings
    ADD CONSTRAINT integration_settings_kind_name_key UNIQUE (kind, name);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Restoring UNIQUE (kind) is impossible once the feature has been used, and a
-- down that dies half-applied under incident pressure is the worst outcome. So
-- it refuses explicitly, naming the kinds that block it.
--
-- The HINT points at user_identities BEFORE deleting anything: "keep one per
-- kind" is not actionable when the wrong choice strands every account linked to
-- the provider name that goes.
DO $$
DECLARE
    dupes TEXT;
BEGIN
    SELECT string_agg(kind, ', ') INTO dupes
    FROM (SELECT kind FROM integration_settings GROUP BY kind HAVING count(*) > 1) d;

    IF dupes IS NOT NULL THEN
        RAISE EXCEPTION 'integration_settings holds more than one row for: %. UNIQUE (kind) cannot be restored', dupes
            USING HINT = 'Check user_identities for accounts linked to the provider names you are about to remove, then delete the extra rows (one per kind survives) and re-run.';
    END IF;
END
$$;
-- +goose StatementEnd

ALTER TABLE integration_settings DROP CONSTRAINT IF EXISTS integration_settings_kind_name_key;
ALTER TABLE integration_settings ADD CONSTRAINT integration_settings_kind_key UNIQUE (kind);
ALTER TABLE integration_settings DROP COLUMN name;

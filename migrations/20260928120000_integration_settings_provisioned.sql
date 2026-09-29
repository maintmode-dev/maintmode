-- +goose Up
-- +goose StatementBegin
-- A login provider can now be declared in the config file as well as created
-- through the admin API. The flag says which of the two owns the row: a
-- provisioned row is written by startup provisioning only, refuses every admin
-- write, and stores NO secret -- its client_secret lives in the process that
-- read it from the secrets file.
--
-- A column on the existing row rather than a second table, because the row is
-- still what user_identities.integration_id points at: taking a provider over
-- from the API must keep its id, or every linked account loses its provider.
ALTER TABLE integration_settings
    ADD COLUMN provisioned BOOLEAN NOT NULL DEFAULT false;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Drops the flag and nothing else. Every row that was provisioned -- inserted by
-- provisioning or taken over from the API -- keeps secrets = {}, so on the
-- previous binary it is a provider with no secret: unreadable until an admin
-- PATCHes client_secret back in.
ALTER TABLE integration_settings
    DROP COLUMN provisioned;
-- +goose StatementEnd

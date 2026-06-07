-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';

-- created_by_user_id / updated_by_user_id record who created and last edited a
-- channel, captured from the authenticated user's access token on create/update.
-- The UI renders the catalog detail header ("Created by ... · Updated by ...").
--
-- No foreign key to users(id) on purpose: the users table is owned by the auth
-- service and resolved over S2S, so the authorship identity here is the token
-- subject (user id), not a row that must already exist locally. This mirrors
-- maintenances.created_by_user_id and avoids coupling channel writes to the
-- user-row lifecycle.
--
-- Both columns are nullable: messenger_channels predates this change, so existing
-- catalog rows carry no author; read paths degrade an unresolved/absent id to the
-- "Unknown user" summary rather than failing. New channels always set
-- created_by_user_id (the create path requires an authenticated user).
ALTER TABLE messenger_channels ADD COLUMN created_by_user_id UUID;
ALTER TABLE messenger_channels ADD COLUMN updated_by_user_id UUID;

-- updated_at tracks the last edit (NULL until a channel is first edited), so the
-- detail header can show "Updated by ... · 1h ago" only when an edit happened.
ALTER TABLE messenger_channels ADD COLUMN updated_at TIMESTAMPTZ;

COMMENT ON COLUMN messenger_channels.created_by_user_id IS
    'Author of the channel: the authenticated user who created it (token subject).';
COMMENT ON COLUMN messenger_channels.updated_by_user_id IS
    'Last editor of the channel: the authenticated user of the most recent edit.';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
ALTER TABLE messenger_channels DROP COLUMN updated_at;
ALTER TABLE messenger_channels DROP COLUMN updated_by_user_id;
ALTER TABLE messenger_channels DROP COLUMN created_by_user_id;
-- +goose StatementEnd

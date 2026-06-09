-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';

-- created_by_user_id / updated_by_user_id record who created and last edited a
-- resource, captured from the authenticated user's access token on
-- create/update. The UI renders the resource detail Identity grid ("Created /
-- Last updated · @handle").
--
-- No foreign key to users(id) on purpose: the users table is owned by the auth
-- service and resolved over S2S, so the authorship identity here is the token
-- subject (user id), not a row that must already exist locally. This mirrors
-- maintenances.created_by_user_id and messenger_channels and avoids coupling
-- resource writes to the user-row lifecycle.
--
-- Both columns are nullable: resources predates this change, so existing rows
-- carry no author; read paths degrade an unresolved/absent id to the
-- "Unknown user" summary rather than failing. New resources always set
-- created_by_user_id (the create path requires an authenticated user).
ALTER TABLE resources ADD COLUMN created_by_user_id UUID;
ALTER TABLE resources ADD COLUMN updated_by_user_id UUID;

COMMENT ON COLUMN resources.created_by_user_id IS
    'Author of the resource: the authenticated user who created it (token subject).';
COMMENT ON COLUMN resources.updated_by_user_id IS
    'Last editor of the resource: the authenticated user of the most recent edit.';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
ALTER TABLE resources DROP COLUMN updated_by_user_id;
ALTER TABLE resources DROP COLUMN created_by_user_id;
-- +goose StatementEnd

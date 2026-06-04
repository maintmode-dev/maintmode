-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';

-- created_by_user_id records the author (creator) of a maintenance, captured
-- from the authenticated user's access token when a draft is created. Every
-- maintenance has an author, so the column is NOT NULL — the create path
-- requires an authenticated user, and there are no historical authorless rows
-- to back-fill.
--
-- No foreign key to users(id) on purpose: the author is the identity carried by
-- the access token (subject = user id), which is the source of truth here, not
-- a row that must already exist in users. Keeping the column independent avoids
-- coupling maintenance creation to user-row lifecycle and lets the value stand
-- on its own (e.g. tokens minted out-of-band in tests). Author display data is
-- rendered from the token at create time, not by joining users.
ALTER TABLE maintenances ADD COLUMN created_by_user_id UUID NOT NULL;

COMMENT ON COLUMN maintenances.created_by_user_id IS
    'Author of the maintenance: the authenticated user who created the draft.';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
ALTER TABLE maintenances DROP COLUMN created_by_user_id;
-- +goose StatementEnd

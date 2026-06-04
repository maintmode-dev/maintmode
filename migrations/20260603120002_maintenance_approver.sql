-- +goose Up
-- +goose StatementBegin

-- approver_user_id is the user assigned to approve this maintenance. It is
-- NOT NULL: every maintenance always has an assigned approver. Safe to add as
-- NOT NULL without a default because the table carries no rows in prod yet.
--
-- There is intentionally NO foreign key to users: the users table is owned by
-- the auth service (separate service boundary), and maintmode has no SQL access
-- to it — users are read only over S2S. The selected approver is validated at
-- write time (must be an active reviewer/admin); the read path resolves the
-- approver summary over S2S and tolerates a since-deleted user.
ALTER TABLE maintenances ADD COLUMN approver_user_id UUID NOT NULL;

CREATE INDEX idx_maintenances_approver_user_id ON maintenances (approver_user_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_maintenances_approver_user_id;
ALTER TABLE maintenances DROP COLUMN IF EXISTS approver_user_id;
-- +goose StatementEnd

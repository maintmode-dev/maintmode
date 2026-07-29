-- +goose Up
-- +goose StatementBegin
-- Users tagged alongside the owner in a maintenance notification.
-- No FK to users: that table belongs to the auth module, the same boundary the
-- maintenances.approver_user_id column already respects.
CREATE TABLE maintenance_mentions (
    id             UUID        PRIMARY KEY DEFAULT uuidv7(),
    maintenance_id UUID        NOT NULL REFERENCES maintenances (id) ON DELETE CASCADE,
    user_id        UUID        NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One index only: maintenance_id leads it, so it also serves the read path that
-- fetches every mention of a maintenance.
CREATE UNIQUE INDEX maintenance_mentions_maint_user_idx
    ON maintenance_mentions (maintenance_id, user_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE maintenance_mentions;
-- +goose StatementEnd

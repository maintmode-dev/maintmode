-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';
CREATE TABLE maintenance_notify_targets (
    id              UUID        PRIMARY KEY DEFAULT uuidv7(),
    maintenance_id  UUID        NOT NULL REFERENCES maintenances(id) ON DELETE CASCADE,
    transport       TEXT        NOT NULL,
    channel_id      TEXT        NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX maintenance_notify_targets_uidx
    ON maintenance_notify_targets (maintenance_id, transport, channel_id);

CREATE INDEX maintenance_notify_targets_maint_idx
    ON maintenance_notify_targets (maintenance_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
DROP TABLE maintenance_notify_targets;
-- +goose StatementEnd

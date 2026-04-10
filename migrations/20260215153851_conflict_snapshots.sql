-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';
CREATE TABLE maintenance_conflict_snapshot (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    maintenance_id uuid NOT NULL,
    conflicted_maintenance_id uuid NOT NULL,
    resource_id uuid,
    conflict_period tstzrange NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_maintenance_conflict_snapshot_maintenance_id
    ON maintenance_conflict_snapshot (maintenance_id);

CREATE INDEX idx_maintenance_conflict_snapshot_conflicted_maintenance_id
    ON maintenance_conflict_snapshot (conflicted_maintenance_id);

ALTER TABLE maintenance_conflict_snapshot
    ADD CONSTRAINT fk_maintenance_conflict_snapshot_maintenance_id
    FOREIGN KEY (maintenance_id)
    REFERENCES maintenances(id)
    ON DELETE CASCADE;

ALTER TABLE maintenance_conflict_snapshot
    ADD CONSTRAINT fk_maintenance_conflict_snapshot_conflicted_maintenance_id
    FOREIGN KEY (conflicted_maintenance_id)
    REFERENCES maintenances(id)
    ON DELETE CASCADE;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
DROP TABLE maintenance_conflict_snapshot;
-- +goose StatementEnd

-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';
CREATE TABLE maintenance_steps (
   id                   UUID        PRIMARY KEY DEFAULT uuidv7(),
    maintenance_id      UUID        NOT NULL REFERENCES maintenances(id) ON DELETE CASCADE,
    step_order          INT         NOT NULL,
    description         TEXT        NOT NULL,
    rollback            TEXT        NOT NULL,
    duration_minutes    BIGINT      NOT NULL,
    status              TEXT        NOT NULL,
    actual_started_at   TIMESTAMPTZ,
    actual_completed_at TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ
);

CREATE UNIQUE INDEX maintenance_steps_order_uidx
    ON maintenance_steps (maintenance_id, step_order);

CREATE INDEX maintenance_steps_maint_idx
    ON maintenance_steps (maintenance_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
DROP TABLE maintenance_steps;
-- +goose StatementEnd

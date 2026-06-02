-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';
-- A deferred notification is a reminder scheduled to fire at fire_at relative to
-- a maintenance. It carries only the schedule: the recipients are the
-- maintenance's notify targets (resolved at send time) and the text is rendered
-- from the maint.reminder template, so neither is stored here.
--
-- goque_task_id is filled in on approve, when the reminder is enqueued, so the
-- pending task can be canceled if the maintenance is canceled. It is nullable
-- (NULL while the maintenance is a draft, before any task exists) and
-- intentionally has NO foreign key to the library-owned goque_task table, whose
-- rows are pruned independently.
CREATE TABLE maintenance_deferred_notifications (
    id              UUID        PRIMARY KEY DEFAULT uuidv7(),
    maintenance_id  UUID        NOT NULL REFERENCES maintenances(id) ON DELETE CASCADE,
    fire_at         TIMESTAMPTZ NOT NULL,
    goque_task_id   UUID,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX maintenance_deferred_notifications_maint_idx
    ON maintenance_deferred_notifications (maintenance_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
DROP TABLE maintenance_deferred_notifications;
-- +goose StatementEnd

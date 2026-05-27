-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';
set time zone 'UTC';

CREATE TABLE goque_task (
                            id              UUID        PRIMARY KEY,
                            type            TEXT        NOT NULL,
                            external_id     TEXT        NOT NULL,
                            payload         JSONB       NOT NULL,
                            status          TEXT        NOT NULL,
                            attempts        INT         NOT NULL,
                            errors          TEXT,
                            metadata        JSONB,
                            created_at      TIMESTAMPTZ NOT NULL    DEFAULT now(),
                            updated_at      TIMESTAMPTZ,
                            next_attempt_at TIMESTAMPTZ NOT NULL    DEFAULT now()
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE UNIQUE INDEX goque_task_type_external_id_idx ON goque_task (type, external_id);
CREATE INDEX goque_task_type_status_next_attempt_at_idx ON goque_task (type, status, next_attempt_at ASC);
CREATE INDEX goque_task_type_status_updated_at_idx ON goque_task (type, status, updated_at ASC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
DROP TABLE goque_task;
-- +goose StatementEnd

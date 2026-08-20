-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';
CREATE TABLE maintenances (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    title TEXT NOT NULL,
    description TEXT NOT NULL ,
    -- Planned period (always present)
    planned_period tstzrange NOT NULL,
    -- Actual period (NULL before the start, may be open-ended)
    actual_period tstzrange,

    impact TEXT NOT NULL,
    status TEXT NOT NULL,
    scope TEXT NOT NULL,
    canceled_reason_code text,
    canceled_reason_comment text,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ,

    -- planned_period is always well-formed
    CHECK (
        NOT isempty(planned_period)
        AND lower(planned_period) < upper(planned_period)
        AND upper(planned_period) IS NOT NULL
    ),

    -- actual_period is either NULL or well-formed
    CHECK (
        actual_period IS NULL
        or (
            NOT isempty(actual_period)
            and (
                lower(actual_period) < upper(actual_period)
                or upper(actual_period) IS NULL
            )
        )
    )
);

--  Index for collision detection (core)
CREATE INDEX maint_planned_period_gist ON maintenances USING GIST (planned_period);
--  Index for runtime (what is running right now)
CREATE INDEX maint_actual_period_gist ON maintenances USING GIST (actual_period)
    WHERE actual_period IS NOT NULL;

-- Speeds up status lookups (enough for the MVP)
CREATE INDEX maint_status_idx ON maintenances (status);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
DROP TABLE maintenances;
-- +goose StatementEnd

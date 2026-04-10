-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';
CREATE TABLE maintenances (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    title TEXT NOT NULL,
    description TEXT NOT NULL ,
    -- Плановый период (всегда есть)
    planned_period tstzrange NOT NULL,
    -- Фактический период (NULL до старта, может быть open-ended)
    actual_period tstzrange,

    impact TEXT NOT NULL,
    status TEXT NOT NULL,
    scope TEXT NOT NULL,
    canceled_reason_code text,
    canceled_reason_comment text,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ,

    -- planned_period всегда корректен
    CHECK (
        NOT isempty(planned_period)
        AND lower(planned_period) < upper(planned_period)
        AND upper(planned_period) IS NOT NULL
    ),

    -- actual_period либо NULL, либо корректный
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

--  Индекс для collision detection (core)
CREATE INDEX maint_planned_period_gist ON maintenances USING GIST (planned_period);
--  Индекс для runtime (что сейчас выполняется)
CREATE INDEX maint_actual_period_gist ON maintenances USING GIST (actual_period)
    WHERE actual_period IS NOT NULL;

-- Ускорение выборок по статусу (MVP достаточно)
CREATE INDEX maint_status_idx ON maintenances (status);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
DROP TABLE maintenances;
-- +goose StatementEnd

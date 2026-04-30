-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';
CREATE TABLE resources (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    name TEXT NOT NULL,
    description TEXT NOT NULL ,
    external_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ
);

CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX resources_name_idx ON resources USING gin (name gin_trgm_ops);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
DROP TABLE resources;
-- +goose StatementEnd

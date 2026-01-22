-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';
CREATE TABLE resources (
    id UUID PRIMARY KEY, -- UUIDv7 генерируем в Go
    name TEXT NOT NULL,
    description TEXT NOT NULL ,
    external_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ
);

CREATE INDEX resources_name_idx ON resources (name);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
DROP TABLE resources;
-- +goose StatementEnd

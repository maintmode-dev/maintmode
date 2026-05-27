-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';
CREATE TABLE maintenance_resources (
    maintenance_id UUID NOT NULL REFERENCES maintenances(id) ON DELETE CASCADE,

    resource_id UUID NOT NULL REFERENCES resources(id),

    PRIMARY KEY (maintenance_id, resource_id)
);

CREATE INDEX maint_res_resource_idx ON maintenance_resources (resource_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
DROP TABLE maintenance_resources;
-- +goose StatementEnd

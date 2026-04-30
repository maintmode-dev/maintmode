-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';
CREATE TABLE maintenance_resources (
    maintenance_id UUID NOT NULL REFERENCES maintenances(id) ON DELETE CASCADE,

    resource_type TEXT NOT NULL, -- service | database | cluster | ...

    resource_id UUID NOT NULL REFERENCES resources(id),

    PRIMARY KEY (maintenance_id, resource_type, resource_id)
);

-- Индекс для поиска конфликтов по ресурсам
CREATE INDEX maint_res_lookup ON maintenance_resources (resource_type, resource_id);

CREATE INDEX maint_res_maint_resource_idx ON maintenance_resources (maintenance_id, resource_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
DROP TABLE maintenance_resources;
-- +goose StatementEnd

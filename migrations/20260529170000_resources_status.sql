-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';

-- status has no DB-level default: the resource service is the source of truth
-- and always sets it explicitly (active on create). Existing rows are
-- backfilled to 'active' as a one-time migration step.
ALTER TABLE resources ADD COLUMN status TEXT;
UPDATE resources SET status = 'active' WHERE status IS NULL;
ALTER TABLE resources ALTER COLUMN status SET NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
ALTER TABLE resources DROP COLUMN status;
-- +goose StatementEnd

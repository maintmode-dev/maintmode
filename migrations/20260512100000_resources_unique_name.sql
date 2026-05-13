-- +goose Up
-- +goose StatementBegin
-- Case-insensitive unique constraint on resource names.
-- Sanity check before applying on environments with data:
--   SELECT lower(name), count(*) FROM resources GROUP BY 1 HAVING count(*) > 1;
-- If duplicates exist, deduplicate (or wipe dev/test data) before running.
--
-- Operational note: this uses a plain CREATE UNIQUE INDEX which takes an
-- ACCESS EXCLUSIVE lock and blocks writes for the duration of the build.
-- This is acceptable for the current MVP (small dataset, low traffic).
-- When the resources table grows, switch to CREATE UNIQUE INDEX CONCURRENTLY
-- in a follow-up migration annotated with the goose "NO TRANSACTION" directive,
-- because CONCURRENTLY cannot run inside a transaction block.
CREATE UNIQUE INDEX resources_lower_name_uniq ON resources (lower(name));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS resources_lower_name_uniq;
-- +goose StatementEnd

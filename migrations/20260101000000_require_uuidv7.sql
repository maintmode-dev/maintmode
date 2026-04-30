-- +goose Up
-- +goose StatementBegin
DO $$
BEGIN
    PERFORM uuidv7()::uuid;
EXCEPTION
    WHEN undefined_function THEN
        RAISE EXCEPTION 'uuidv7() function is required before running MaintMode migrations'
            USING HINT = 'Use a supported PostgreSQL image with uuidv7(), such as postgres:18-alpine or later.';
END
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'uuidv7() prerequisite check cannot be rolled back';
-- +goose StatementEnd

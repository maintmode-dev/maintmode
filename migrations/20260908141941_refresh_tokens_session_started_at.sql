-- +goose Up
-- +goose StatementBegin
-- Session length becomes an instance policy rather than a per-login choice, so
-- the row records when the SESSION began instead of how long it asked to live.
--
-- created_at cannot serve that purpose: rotation inserts a NEW row, so it marks
-- the last rotation, not the login. With both timestamps present, two
-- independent questions get answered -- "has this session existed too long?"
-- against session_started_at, and "has it been idle too long?" against
-- created_at.
--
-- Neither deadline is stored precomputed. Changing the policy therefore takes
-- effect on sessions that already exist, not only on ones created afterwards.
ALTER TABLE refresh_tokens
    ADD COLUMN session_started_at TIMESTAMPTZ NOT NULL DEFAULT now();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE refresh_tokens
    DROP COLUMN session_started_at;
-- +goose StatementEnd

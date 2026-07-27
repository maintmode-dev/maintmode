-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';

-- The 64-character cap already lives in the domain validation, and both write
-- paths — the self-service patch and the admin one — go through it, so nothing
-- reachable through the API can exceed it today. This constraint is a backstop
-- for what that validation cannot see: a value written by direct SQL or by a
-- future migration ends up pasted verbatim into an outgoing notification, and
-- the render-time sanitizer inspects characters, not length.
ALTER TABLE users
    ADD CONSTRAINT users_telegram_tag_length CHECK (length(telegram_tag) <= 64),
    ADD CONSTRAINT users_slack_tag_length CHECK (length(slack_tag) <= 64);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
ALTER TABLE users
    DROP CONSTRAINT users_telegram_tag_length,
    DROP CONSTRAINT users_slack_tag_length;
-- +goose StatementEnd

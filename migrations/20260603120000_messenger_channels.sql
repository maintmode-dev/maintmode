-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';
CREATE TABLE messenger_channels (
    id                   UUID        PRIMARY KEY DEFAULT uuidv7(),
    transport            TEXT        NOT NULL CHECK (transport IN ('slack', 'telegram')),
    transport_channel_id TEXT        NOT NULL,
    name                 TEXT        NOT NULL,
    description          TEXT        NOT NULL,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- archived_at soft-deletes a channel: it disappears from the default
    -- GET /channels listing but stays resolvable by id so existing
    -- subscription validation does not break. NULL means active.
    archived_at          TIMESTAMPTZ
);

CREATE UNIQUE INDEX messenger_channels_transport_channel_uidx
    ON messenger_channels (transport, transport_channel_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
DROP TABLE messenger_channels;
-- +goose StatementEnd

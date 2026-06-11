-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';
-- Re-point maintenance_notify_targets at the channel catalog (RUK-177).
-- Targets used to snapshot the raw (transport, channel_id) delivery pair —
-- the table predates the messenger_channels catalog — which left them without
-- referential integrity and orphaned them whenever a channel's
-- transport_channel_id was edited. Now a target references
-- messenger_channels.id and the delivery address is resolved from the catalog
-- at dispatch time (live reference: editing a channel redirects notifications).
-- No production data exists, so the table is recreated instead of migrated.
DROP TABLE maintenance_notify_targets;

CREATE TABLE maintenance_notify_targets (
    id              UUID        PRIMARY KEY DEFAULT uuidv7(),
    maintenance_id  UUID        NOT NULL REFERENCES maintenances (id) ON DELETE CASCADE,
    channel_id      UUID        NOT NULL REFERENCES messenger_channels (id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX maintenance_notify_targets_uidx
    ON maintenance_notify_targets (maintenance_id, channel_id);

CREATE INDEX maintenance_notify_targets_maint_idx
    ON maintenance_notify_targets (maintenance_id);

-- The calendar/related-maintenances filter looks targets up by channel alone
-- (WHERE channel_id = ANY(...)), and FK enforcement benefits from it too —
-- neither the unique index (maintenance_id-leading) nor the maint index
-- covers that access path.
CREATE INDEX maintenance_notify_targets_channel_idx
    ON maintenance_notify_targets (channel_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
DROP TABLE maintenance_notify_targets;

CREATE TABLE maintenance_notify_targets (
    id              UUID        PRIMARY KEY DEFAULT uuidv7(),
    maintenance_id  UUID        NOT NULL REFERENCES maintenances (id) ON DELETE CASCADE,
    transport       TEXT        NOT NULL,
    channel_id      TEXT        NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX maintenance_notify_targets_uidx
    ON maintenance_notify_targets (maintenance_id, transport, channel_id);

CREATE INDEX maintenance_notify_targets_maint_idx
    ON maintenance_notify_targets (maintenance_id);
-- +goose StatementEnd

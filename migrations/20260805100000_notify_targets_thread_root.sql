-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';

-- root_message_id holds the reference to the "maintenance started" message
-- delivered to this channel: subsequent lifecycle events are sent as replies
-- to it. It is the only value a reply is addressed by.
--
-- TEXT rather than a numeric type: Slack ts is the string "1503435956.000247",
-- and a trip through float silently breaks threading (the API answers ok:true
-- and the message lands outside the thread).
--
-- root_channel is the delivery address the root was actually sent to, copied
-- from the catalog at start time. The catalog address is editable at runtime,
-- so comparing it against the channel's current address is what detects a
-- re-pointed channel — a root from the old chat must not be replayed into the
-- new one, where that message id belongs to strangers. The comparison has to
-- be address-to-address: the canonical id a messenger API answers with is
-- written by a different system than this free-form operator text, so the two
-- legitimately differ. That canonical id is logged at delivery rather than
-- stored, since nothing reads it back.
--
-- Nullable with no backfill: the root does not exist before the start, and
-- maintenances already running will not have one — they live out flat.
ALTER TABLE maintenance_notify_targets
    ADD COLUMN root_message_id TEXT,
    ADD COLUMN root_channel    TEXT;

COMMENT ON COLUMN maintenance_notify_targets.root_message_id IS
    'Message id of the delivered "maintenance started" notification; subsequent lifecycle events reply to it.';
COMMENT ON COLUMN maintenance_notify_targets.root_channel IS
    'Delivery address the root was actually sent to, copied from the catalog at start time. Compared against the catalog''s current address to detect a re-pointed channel.';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';

ALTER TABLE maintenance_notify_targets
    DROP COLUMN root_message_id,
    DROP COLUMN root_channel;
-- +goose StatementEnd

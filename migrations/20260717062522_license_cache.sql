-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';

-- license_cache holds the last successful heartbeat response from the MaintMode
-- Console (SaaS mode). Singleton row (id is a bool locked to TRUE):
-- one instance = one organization, so there is exactly one license. When
-- Console is unreachable the instance keeps operating on this cached license
-- indefinitely — an admin-panel outage must never degrade a customer.
-- Self-hosted deployments never write here (license client disabled).
CREATE TABLE license_cache (
    id              BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
    -- status: active | blocked, as sent by Console. blocked rejects every
    -- mutating request on the instance. Seeded active (see below) and always
    -- present, so the block gate never faces a missing status.
    status          TEXT NOT NULL,
    -- seats_purchased and fetched_at are Console-owned: NULL until the first
    -- heartbeat fills them. NULL honestly means "Console has not reported this
    -- yet" rather than a fake zero seat count or a fake confirmation time.
    seats_purchased BIGINT,
    -- fetched_at: when this license was last confirmed by Console.
    fetched_at      TIMESTAMPTZ
);

-- Seed the singleton with a default active license so the block gate has a row
-- to read from the very first request — before the first cron heartbeat lands.
-- Fail-open by default: a fresh instance is never blocked on a missing Console
-- answer. seats_purchased and fetched_at stay NULL until the first heartbeat.
-- On self-hosted the row is inert — the license client is disabled, gate is Noop.
INSERT INTO license_cache (id, status)
VALUES (TRUE, 'active');

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
DROP TABLE license_cache;
-- +goose StatementEnd

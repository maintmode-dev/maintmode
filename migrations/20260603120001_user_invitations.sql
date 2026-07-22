-- +goose Up
-- +goose StatementBegin

-- user_invitations stores admin-issued invitations to onboard new users.
-- The raw invitation token is only ever delivered in the email link; the DB
-- stores its sha256 hash. Lookups (preview/accept) hash the incoming token and
-- match by token_hash, so a DB read never reveals a usable token.
--
-- Stored status is one of pending|accepted|revoked|expired. "expired" is DERIVED
-- at read time from (status='pending' AND expires_at < now()) AND — since
-- RUK-211 — also PERSISTED by the invitation-rotation sweep, which flips pending
-- rows past expiry to status='expired'. Either way an expired row leaves the
-- partial-unique "one active pending per email" index (WHERE status='pending'),
-- so the "one active pending per email" invariant still holds and the email slot
-- is freed for a fresh invite.
CREATE TABLE user_invitations (
    id            UUID        PRIMARY KEY DEFAULT uuidv7(),
    email         TEXT        NOT NULL,
    roles         TEXT[]      NOT NULL DEFAULT '{}',
    token_hash    TEXT        NOT NULL, -- sha256 hex of the raw token (raw only in the email link)
    status        TEXT        NOT NULL DEFAULT 'pending', -- pending|accepted|revoked|expired (expired derived on read + persisted by rotation, RUK-211)
    invited_by_id UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at    TIMESTAMPTZ NOT NULL,
    sent_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    accepted_at   TIMESTAMPTZ,
    revoked_at    TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Token lookup for preview/accept (by hash, never raw).
CREATE UNIQUE INDEX user_invitations_token_hash_uidx ON user_invitations (token_hash);

-- "One active pending invitation per email": a partial unique index over
-- lower(email) restricted to status='pending'. Re-inviting an email with a live
-- pending invitation is handled at the service layer by revoking the old
-- pending row first (so this index never blocks a re-invite), and accepted /
-- revoked rows do not participate in uniqueness.
CREATE UNIQUE INDEX user_invitations_active_pending_uidx
    ON user_invitations (lower(email))
    WHERE status = 'pending';

CREATE INDEX idx_user_invitations_status ON user_invitations (status);
CREATE INDEX idx_user_invitations_email ON user_invitations (lower(email));
CREATE INDEX idx_user_invitations_invited_by_id ON user_invitations (invited_by_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS user_invitations;
-- +goose StatementEnd

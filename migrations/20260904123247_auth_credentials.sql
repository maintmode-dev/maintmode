-- +goose Up
-- +goose StatementBegin

-- auth_credentials stores the secrets for the two built-in sign-in methods:
-- a one-time code sent by email, and a password. Both are rows of this one
-- table, distinguished by `kind` -- they are the same primitive (a user secret
-- held as a hash), and keeping them together means the two sign-in methods need
-- one migration rather than two competing ones.
--
-- The raw secret is never stored. For an OTP the column holds a sha256 hex
-- digest; for a password it holds an argon2id PHC string. Two different hash
-- functions in one column is deliberate: an OTP lives for minutes so hash cost
-- is irrelevant, while a password lives for years and hash cost is the point.
--
-- Two mechanisms stop the two from being confused, and both are required.
-- First, `kind` participates in the partial unique indexes below and in every
-- query predicate, so a password row cannot surface on an OTP read path through
-- a normal query. Second, the hash is self-describing: "$argon2id$..." versus
-- bare hex, so a verifier reads the algorithm out of the data itself and fails
-- loudly on a mismatch instead of quietly checking a password as sha256.
--
-- ROLLING THIS BACK DESTROYS DATA. Today the table is empty and has no
-- consumers, so Down is harmless. Once the sign-in methods ship, this table
-- holds the only copy of every user's password hash and there is no way to
-- reconstruct it -- unlike an invitation, which can simply be re-sent. Walking
-- migrations back past this point is a password-loss event, not a schema
-- revert.
CREATE TABLE auth_credentials (
    id            UUID        PRIMARY KEY DEFAULT uuidv7(),
    user_id       UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind          TEXT        NOT NULL, -- 'password' | 'otp'
    secret_hash   TEXT        NOT NULL, -- argon2id PHC for password, sha256 hex for otp
    expires_at    TIMESTAMPTZ,          -- NULL for password
    consumed_at   TIMESTAMPTZ,          -- single-use marker for otp
    attempts      SMALLINT    NOT NULL DEFAULT 0,
    session_nonce TEXT,                 -- binds an otp to one browser; NULL for password
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- `kind` drives a partial unique index and every query predicate, so a
    -- typo'd value would create a row participating in NO uniqueness index at
    -- all: invisible to every read path, and silently breaking the "one
    -- password / one live OTP per user" invariant without raising anything.
    CONSTRAINT auth_credentials_kind_check CHECK (kind IN ('password', 'otp'))
);

-- A user has exactly one password.
CREATE UNIQUE INDEX auth_credentials_password_uidx
    ON auth_credentials (user_id) WHERE kind = 'password';

-- A user has at most one live OTP: issuing a new one consumes the previous.
-- Consuming a code clears it from this index and frees the slot, the same way
-- user_invitations_active_pending_uidx frees an email slot on acceptance.
CREATE UNIQUE INDEX auth_credentials_active_otp_uidx
    ON auth_credentials (user_id) WHERE kind = 'otp' AND consumed_at IS NULL;

-- Supports the periodic prune of expired codes, which is a separate piece of
-- work; only the index it will read lands here.
CREATE INDEX auth_credentials_otp_expiry_idx
    ON auth_credentials (expires_at) WHERE kind = 'otp';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Refuse to drop a populated table. The warning above is one a human has to
-- read; this is one the database enforces, and rolling back under incident
-- pressure is exactly when the comment goes unread.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM auth_credentials LIMIT 1) THEN
        RAISE EXCEPTION 'auth_credentials is not empty: rolling back destroys every stored password hash'
            USING HINT = 'Back it up first. If the loss is genuinely intended, TRUNCATE the table and re-run.';
    END IF;
END
$$;

DROP TABLE IF EXISTS auth_credentials;
-- +goose StatementEnd

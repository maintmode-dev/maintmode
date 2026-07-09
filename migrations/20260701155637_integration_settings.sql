-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';
-- integration_settings is the type-independent registry of external-system
-- connections (Slack, Telegram, SMTP; later Jira/etc), managed by an admin
-- through the UI so integrations can be toggled/reconfigured at runtime without a
-- redeploy (RUK-196). One row per kind (UNIQUE(kind)); adding a new integration
-- type is a new kind value plus a small parser/validator, no DDL.
--
-- config holds NON-secret fields (host, port, from, tls_policy, api_url, timeout)
-- as plaintext jsonb so they show in the UI and diagnostics without decryption.
-- secrets holds ONLY the secret fields ({"<key>": "<base64(envelope)>"}), each
-- value encrypted with the DEK referenced by dek_id; the plaintext secret never
-- lives here.
CREATE TABLE integration_settings (
    id                 UUID        PRIMARY KEY DEFAULT uuidv7(),
    -- No CHECK enum on kind (unlike messenger_channels.transport): a new
    -- integration type must be a new kind value plus a validator, with no DDL.
    -- Allowed kinds are enforced by the Integration registry, not the schema.
    kind               TEXT        NOT NULL,
    enabled            BOOLEAN     NOT NULL,
    config             JSONB       NOT NULL DEFAULT '{}'::jsonb,
    secrets            JSONB       NOT NULL DEFAULT '{}'::jsonb,
    -- ON DELETE RESTRICT: a DEK that still protects live integration secrets must
    -- not be deletable out from under them (would make the secrets unreadable).
    dek_id             UUID        NOT NULL REFERENCES data_keys (id) ON DELETE RESTRICT,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Authorship captured from the access token; no FK to users (owned by auth,
    -- resolved on read), mirroring messenger_channels / resources.
    created_by_user_id UUID,
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by_user_id UUID,
    -- One integration per kind for now. Multi-connection (UNIQUE(kind, name)) is
    -- an additive future change.
    UNIQUE (kind)
);

COMMENT ON COLUMN integration_settings.kind IS
    'Integration type: telegram | slack | smtp | jira | ... — matches messenger_channels.transport for user-subscribable kinds.';
COMMENT ON COLUMN integration_settings.enabled IS
    'Runtime on/off toggle read by the transport resolver; a disabled integration drops delivery best-effort.';
COMMENT ON COLUMN integration_settings.config IS
    'Non-secret settings as plaintext jsonb (host, port, from, tls_policy, api_url, timeout, ...).';
COMMENT ON COLUMN integration_settings.secrets IS
    'Secret fields only, each value base64(Tink AEAD envelope) encrypted with the DEK at dek_id. Never plaintext.';
COMMENT ON COLUMN integration_settings.dek_id IS
    'The data_keys row whose DEK encrypts this integration''s secret values.';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
-- Rolling this back drops every integration secret. It also releases the FK to
-- data_keys, so a subsequent rollback of the data_keys migration becomes possible
-- again — and dropping data_keys destroys the DEKs, making any secrets that were
-- encrypted with them permanently unrecoverable. Do not roll back past this in an
-- environment holding real integration secrets.
DROP TABLE integration_settings;
-- +goose StatementEnd

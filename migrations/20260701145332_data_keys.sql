-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';
-- data_keys stores the data-encryption keys (DEKs) that protect integration
-- secrets at rest. Each DEK is stored wrapped (envelope-encrypted) by a KEK, so
-- the plaintext DEK never touches the database. kek_id records which KEK wrapped
-- the DEK so a KEK rotation can re-wrap it and fail-fast can detect an unknown
-- KEK. This is the crypto-low foundation for RUK-196; the integration registry
-- references a row here via integration_settings.dek_id.
CREATE TABLE data_keys (
    id            UUID        PRIMARY KEY DEFAULT uuidv7(),
    kek_id        TEXT        NOT NULL,
    encrypted_dek BYTEA       NOT NULL,
    label         TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON COLUMN data_keys.kek_id IS
    'URI of the KEK that wrapped encrypted_dek (a key in config crypto.local_keys, e.g. local-kms://...); recorded so a rotation can re-wrap and fail-fast can reject an unknown KEK.';
COMMENT ON COLUMN data_keys.encrypted_dek IS
    'DEK keyset wrapped by the KEK (Tink encrypted keyset bytes). The plaintext DEK never touches the database.';
COMMENT ON COLUMN data_keys.label IS
    'Optional human-readable note for the key (e.g. "integration secrets").';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
DROP TABLE data_keys;
-- +goose StatementEnd

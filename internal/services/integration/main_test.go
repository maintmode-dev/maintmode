package integration_test

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"

	"github.com/lib/pq"

	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/integrationkinds"
	"github.com/ruko1202/maintmode/internal/pkg/secrets"
	integrationsvc "github.com/ruko1202/maintmode/internal/services/integration"
	datakeystore "github.com/ruko1202/maintmode/internal/storages/datakey"
	integrationstore "github.com/ruko1202/maintmode/internal/storages/integration"

	"github.com/ruko1202/maintmode/internal/utils/closer"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"
	publishermock "github.com/ruko1202/maintmode/test/utils/mocks/publisher"
)

var (
	db         *sqlx.DB
	keyring    *secrets.Keyring
	testCipher secrets.AESCipher
)

func TestMain(m *testing.M) {
	db = testdbconnutils.NewDB(config.LoadAppConfig())
	closer.Add(db.Close)

	var err error
	keyring, err = secrets.NewLocalKeyring("local-kms://test-1", map[string]string{"local-kms://test-1": testKEK()})
	if err != nil {
		panic(err)
	}
	testCipher = secrets.NewAESCipher()

	os.Exit(m.Run())
}

// renamedKind wraps a real integrationkinds.Integration under a fresh kind name,
// so each parallel test can register the same behavior (secret keys, parse,
// validate) under a UNIQUE(kind) row of its own — no shared "slack" row to race
// over, no freshSlack cleanup.
type renamedKind struct {
	integrationkinds.Integration
	name string
}

func (r renamedKind) Kind() string { return r.name }

// serviceMocks bundles the per-test doubles a service is wired with.
type serviceMocks struct {
	audit *publishermock.Spy
}

// testKinds are the unique per-test kind names, each backed by the matching real
// integration behavior. They replace the hard-coded "slack"/"email" strings so
// parallel tests never collide on UNIQUE(kind).
type testKinds struct {
	slack    string
	email    string
	telegram string
}

// initService builds a service on the shared DB but with a fresh registry whose
// kinds are uniquely named for this test, plus a per-test audit spy. It returns
// the service, the unique kind names, and the mocks.
func initService(t *testing.T) (*integrationsvc.Service, testKinds, *serviceMocks) {
	t.Helper()

	suffix := "-" + xuuid.NewString()
	kinds := testKinds{
		slack:    integrationkinds.Slack.Kind() + suffix,
		email:    integrationkinds.Email.Kind() + suffix,
		telegram: integrationkinds.Telegram.Kind() + suffix,
	}

	registry, err := integrationsvc.NewRegistry(
		renamedKind{Integration: integrationkinds.Slack, name: kinds.slack},
		renamedKind{Integration: integrationkinds.Email, name: kinds.email},
		renamedKind{Integration: integrationkinds.Telegram, name: kinds.telegram},
	)
	require.NoError(t, err)

	mocks := &serviceMocks{audit: publishermock.New(t)}

	svc := integrationsvc.NewService(
		dbtx.NewTxManager(db),
		integrationstore.NewStore(db),
		datakeystore.NewStore(db),
		registry,
		keyring,
		testCipher,
		mocks.audit,
	)
	// Each test's rows use unique kinds; drop them at the end so the shared table
	// does not accumulate across a package run.
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM integration_settings WHERE kind = ANY($1)`,
			pq.Array([]string{kinds.slack, kinds.email, kinds.telegram}))
	})

	return svc, kinds, mocks
}

// testActor is the authenticated admin performing an operation in a test.
func testActor() *entity.User {
	return &entity.User{ID: uuid.New(), Email: "admin@test.local", Name: "Admin"}
}

// lastUpdated returns the single IntegrationUpdated among the captured actions,
// failing if there is not exactly one. A test that both creates and updates
// through one per-test spy sees the create's IntegrationCreated too, so it asks
// for the update event by type rather than by position/count.
func lastUpdated(t *testing.T, actions []audit.Action) audit.IntegrationUpdated {
	t.Helper()
	var found []audit.IntegrationUpdated
	for _, a := range actions {
		if u, ok := a.(audit.IntegrationUpdated); ok {
			found = append(found, u)
		}
	}
	require.Len(t, found, 1, "exactly one IntegrationUpdated must be published")
	return found[0]
}

// secretsJSON encodes a plaintext secrets map into the raw JSON form the cmd
// carries (the API layer passes secrets through untyped).
func secretsJSON(t *testing.T, m map[string]string) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(m)
	require.NoError(t, err)
	return raw
}

// secretIntentsJSON encodes per-key update intents (string=replace, nil=clear,
// absent=keep) into the raw JSON form UpdateIntegrationCmd carries.
func secretIntentsJSON(t *testing.T, m map[string]*string) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(m)
	require.NoError(t, err)
	return raw
}

func testKEK() string {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	return hex.EncodeToString(key)
}

// rawStoredSecret reads the stored (ciphertext) secret value straight from the
// DB so a test can assert the persisted value is NOT the plaintext input.
//
//nolint:unparam // kind is fixed in current tests but kept for call-site clarity.
func rawStoredSecret(ctx context.Context, t *testing.T, kind, key string) string {
	t.Helper()
	var stored string
	err := db.QueryRowxContext(ctx,
		`SELECT secrets ->> $1 FROM integration_settings WHERE kind = $2`, key, kind).Scan(&stored)
	require.NoError(t, err)
	return stored
}

// rawStoredDEKID returns the dek_id an integration references, so a test can
// assert the DEK is reused (not repointed) across an update.
//
//nolint:unparam // kind is fixed in current tests but kept for call-site clarity.
func rawStoredDEKID(ctx context.Context, t *testing.T, kind string) uuid.UUID {
	t.Helper()
	var dekID uuid.UUID
	err := db.QueryRowxContext(ctx,
		`SELECT dek_id FROM integration_settings WHERE kind = $1`, kind).Scan(&dekID)
	require.NoError(t, err)
	return dekID
}

// decryptStoredSecret reconstructs the decrypt path (unwrap DEK, open envelope)
// so a test can prove the persisted ciphertext still decrypts to the expected
// plaintext — the one round-trip the mask-only read path cannot verify.
//
//nolint:unparam // kind is fixed in current tests but kept for call-site clarity.
func decryptStoredSecret(ctx context.Context, t *testing.T, kind, key string) string {
	t.Helper()

	var encryptedDEK []byte
	var kekID, storedSecret string
	err := db.QueryRowxContext(ctx, `
		SELECT dk.encrypted_dek, dk.kek_id, s.secrets ->> $1
		FROM integration_settings s JOIN data_keys dk ON dk.id = s.dek_id
		WHERE s.kind = $2`, key, kind).Scan(&encryptedDEK, &kekID, &storedSecret)
	require.NoError(t, err)

	dek, err := keyring.UnwrapDEK(encryptedDEK, kekID)
	require.NoError(t, err)
	envelope, err := base64.StdEncoding.DecodeString(storedSecret)
	require.NoError(t, err)
	// The stored secret is bound to its (kind, key) slot via AAD, so decrypt must
	// supply the same AAD the service used to seal it.
	plain, err := testCipher.Decrypt(dek, envelope, secrets.SecretAAD(kind, key))
	require.NoError(t, err)
	return string(plain)
}

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
	"github.com/ruko1202/maintmode/internal/storages/useridentities"

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

func (r renamedKind) Name() string { return r.name }

// PresetKey forwards the wrapped entry's catalog key when it has one.
//
// The embedded interface would carry it implicitly, but only if the wrapper
// itself satisfied Preseted -- and a wrapper over a non-preseted entry must NOT.
// Forwarding explicitly keeps "is this preset-backed" an answer from the real
// entry rather than a property of being wrapped.
func (r renamedKind) PresetKey() string {
	preseted, ok := r.Integration.(integrationkinds.Preseted)
	if !ok {
		return ""
	}

	return preseted.PresetKey()
}

// serviceMocks bundles the per-test doubles a service is wired with.
type serviceMocks struct {
	audit      *publishermock.Spy
	identities *fakeIdentities
}

// fakeIdentities stands in for the auth module's identity store. linked is how
// many identities the cascade will report removing; a test sets it to make a
// provider look "in use".
type fakeIdentities struct {
	linked int64
	// unlinkedFrom is the registry row id the cascade deleted by, recorded so a
	// test can prove the delete addressed the row rather than the category --
	// and, since the id is the row's, that it addressed the RIGHT row.
	unlinkedFrom uuid.UUID
}

// DeleteByIntegrationID stands in for the cascade: it reports what it would
// have removed and records the id, so a test can assert both.
func (f *fakeIdentities) DeleteByIntegrationID(_ context.Context, integrationID uuid.UUID) (int64, error) {
	f.unlinkedFrom = integrationID
	removed := f.linked
	f.linked = 0

	return removed, nil
}

// testKinds are the unique per-test SYSTEM NAMES, each backed by the matching
// real integration behavior. They replace the hard-coded "slack"/"email"
// strings so parallel tests never collide on UNIQUE (kind, name).
//
// The uniqueness moved with the addressing: it used to be the kind that was
// suffixed, because the kind was the system. Now the kind is a category shared
// by every row of its half, so the suffix belongs on the name -- and notify and
// login below are the real categories, not per-test values.
type testKinds struct {
	slack    string
	email    string
	telegram string
	// oidc is the per-test login name backed by a catalog entry: it behaves
	// like "google", so the preset owns issuer_url and display_name and a create
	// supplying either is refused.
	oidc string
	// byoIssuer is the per-test login name with NO catalog entry, registered
	// under a name the preset rules let through. Tests that re-point an issuer
	// need it: under a preset that field cannot be changed at all, so they would
	// otherwise be testing the preset rule instead of the guard they are about.
	byoIssuer string
	// notify and login are the real categories, the same for every test. They
	// live here so a call site reads one pair out of one fixture.
	notify string
	login  string
}

// initService builds a service on the shared DB but with a fresh registry whose
// kinds are uniquely named for this test, plus a per-test audit spy. It returns
// the service, the unique kind names, and the mocks.
func initService(t *testing.T) (*integrationsvc.Service, testKinds, *serviceMocks) {
	t.Helper()

	return initServiceWith(t, nil)
}

// initServiceWithRealIdentities wires the REAL identity store instead of the
// fake, so the foreign key is in the path.
//
// Only the delete-order test needs this. The fake has no database and therefore
// cannot refuse anything, which is exactly what hides a cascade running after
// the row it was supposed to clear.
func initServiceWithRealIdentities(t *testing.T) (*integrationsvc.Service, testKinds, *serviceMocks) {
	t.Helper()

	return initServiceWith(t, useridentities.NewStore(db))
}

// initServiceWith builds the service, using identities when given and the
// recording fake otherwise.
func initServiceWith(
	t *testing.T, identities integrationsvc.IdentitiesStore,
) (*integrationsvc.Service, testKinds, *serviceMocks) {
	t.Helper()

	suffix := "-" + xuuid.NewString()
	kinds := testKinds{
		slack:     integrationkinds.Slack.Name() + suffix,
		email:     integrationkinds.Email.Name() + suffix,
		telegram:  integrationkinds.Telegram.Name() + suffix,
		oidc:      integrationkinds.Google.Name() + suffix,
		byoIssuer: integrationkinds.Custom.Name() + suffix,
		notify:    integrationkinds.CategoryNotify,
		login:     integrationkinds.CategoryLogin,
	}

	registry, err := integrationsvc.NewRegistry(
		renamedKind{Integration: integrationkinds.Slack, name: kinds.slack},
		renamedKind{Integration: integrationkinds.Email, name: kinds.email},
		renamedKind{Integration: integrationkinds.Telegram, name: kinds.telegram},
		renamedKind{Integration: integrationkinds.Google, name: kinds.oidc},
		renamedKind{Integration: integrationkinds.Custom, name: kinds.byoIssuer},
	)
	require.NoError(t, err)

	mocks := &serviceMocks{audit: publishermock.New(t), identities: &fakeIdentities{}}

	if identities == nil {
		identities = mocks.identities
	}

	svc := integrationsvc.NewService(
		dbtx.NewTxManager(db),
		integrationstore.NewStore(db),
		datakeystore.NewStore(db),
		registry,
		keyring,
		testCipher,
		mocks.audit,
	).WithIdentities(identities).
		// The fixture's login name carries the per-test suffix so parallel runs
		// stay off each other's rows, which makes it a PRESET name rather than
		// "custom" -- so it needs a catalog entry to be creatable at all.
		WithLoginPresets(config.LoginPresets{
			integrationkinds.Google.Name(): {DisplayName: "Test IdP", IssuerURL: testPresetIssuer},
		})
	// Each test's rows use unique NAMES; drop them at the end so the shared table
	// does not accumulate across a package run.
	//
	// The DEKs go too. Create mints one per integration, and rotation scans the
	// WHOLE data_keys table under a table-wide FOR UPDATE -- so rows left behind
	// do not just accumulate, they slow every rotation test on the shared dev
	// database until one times out. Deleted after the settings, which reference
	// them.
	t.Cleanup(func() {
		nameList := pq.Array([]string{kinds.slack, kinds.email, kinds.telegram, kinds.oidc, kinds.byoIssuer})

		var dekIDs []string
		rows, err := db.Query(`SELECT dek_id FROM integration_settings WHERE name = ANY($1)`, nameList)
		if err == nil {
			for rows.Next() {
				var id string
				if rows.Scan(&id) == nil {
					dekIDs = append(dekIDs, id)
				}
			}
			_ = rows.Close()
		}

		_, _ = db.Exec(`DELETE FROM integration_settings WHERE name = ANY($1)`, nameList)
		if len(dekIDs) > 0 {
			_, _ = db.Exec(`DELETE FROM data_keys WHERE id = ANY($1)`, pq.Array(dekIDs))
		}
	})

	return svc, kinds, mocks
}

// testPresetIssuer is the issuer the fixture's catalog supplies. A login row
// created through initService carries THIS issuer, not whatever the test passed
// in config -- the preset owns that field.
const testPresetIssuer = "https://idp.fixture.example"

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
func rawStoredSecret(ctx context.Context, t *testing.T, name, key string) string {
	t.Helper()
	var stored string
	// Addressed by NAME. The kind is a shared category now, so selecting on it
	// would match every row this test created and make the result arbitrary.
	err := db.QueryRowxContext(ctx,
		`SELECT secrets ->> $1 FROM integration_settings WHERE name = $2`, key, name).Scan(&stored)
	require.NoError(t, err)
	return stored
}

// rawStoredDEKID returns the dek_id an integration references, so a test can
// assert the DEK is reused (not repointed) across an update.
//
//nolint:unparam // name is fixed in current tests but kept for call-site clarity.
func rawStoredDEKID(ctx context.Context, t *testing.T, name string) uuid.UUID {
	t.Helper()
	var dekID uuid.UUID
	err := db.QueryRowxContext(ctx,
		`SELECT dek_id FROM integration_settings WHERE name = $1`, name).Scan(&dekID)
	require.NoError(t, err)
	return dekID
}

// decryptStoredSecret reconstructs the decrypt path (unwrap DEK, open envelope)
// so a test can prove the persisted ciphertext still decrypts to the expected
// plaintext — the one round-trip the mask-only read path cannot verify.
//
//nolint:unparam // name is fixed in current tests but kept for call-site clarity.
func decryptStoredSecret(ctx context.Context, t *testing.T, name, key string) string {
	t.Helper()

	var encryptedDEK []byte
	var kekID, storedSecret string
	err := db.QueryRowxContext(ctx, `
		SELECT dk.encrypted_dek, dk.kek_id, s.secrets ->> $1
		FROM integration_settings s JOIN data_keys dk ON dk.id = s.dek_id
		WHERE s.name = $2`, key, name).Scan(&encryptedDEK, &kekID, &storedSecret)
	require.NoError(t, err)

	dek, err := keyring.UnwrapDEK(encryptedDEK, kekID)
	require.NoError(t, err)
	envelope, err := base64.StdEncoding.DecodeString(storedSecret)
	require.NoError(t, err)
	// The stored secret is bound to its (system name, key) slot via AAD, so
	// decrypt must supply the same AAD the service used to seal it -- and the
	// service seals with the registry key, which is the name.
	plain, err := testCipher.Decrypt(dek, envelope, secrets.SecretAAD(name, key))
	require.NoError(t, err)
	return string(plain)
}

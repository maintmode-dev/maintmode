package transportresolver_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/gateways/notifytransport"
	"github.com/ruko1202/maintmode/internal/integrationkinds"
	"github.com/ruko1202/maintmode/internal/pkg/secrets"
	integrationsvc "github.com/ruko1202/maintmode/internal/services/integration"
	"github.com/ruko1202/maintmode/internal/services/transportresolver"
	datakeystore "github.com/ruko1202/maintmode/internal/storages/datakey"
	integrationstore "github.com/ruko1202/maintmode/internal/storages/integration"
	"github.com/ruko1202/maintmode/internal/utils/closer"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"
	publishermock "github.com/ruko1202/maintmode/test/utils/mocks/publisher"
)

// The resolver tests run end-to-end: a real integration registry service over
// the shared test DB, wired to the resolver exactly as bootstrap wires them
// (builders + onChange -> Invalidate). This is deliberate — the behaviors under
// test (write invalidates delivery cache, disabled drops, rotate rebuilds) span
// the registry/resolver seam.
var (
	db      *sqlx.DB
	keyring *secrets.Keyring
)

func TestMain(m *testing.M) {
	db = testdbconnutils.NewDB(config.LoadAppConfig())
	closer.Add(db.Close)

	var err error
	keyring, err = secrets.NewLocalKeyring("local-kms://resolver-test-1",
		map[string]string{"local-kms://resolver-test-1": testKEK()})
	if err != nil {
		panic(err)
	}

	os.Exit(m.Run())
}

// harness is one test's isolated registry+resolver pair, wired the same way
// bootstrap wires production (builders + onChange -> Invalidate), plus the unique
// kind name this test's rows use so parallel tests never collide on UNIQUE(kind).
type harness struct {
	registry *integrationsvc.Service
	resolver *transportresolver.Service
	kind     string
}

// resolve is the delivery-side entry the production code uses
// (TransportResolver.Get), bound to this harness's resolver.
func (h harness) resolve(ctx context.Context, kind string) (notifytransport.Transport, error) {
	return h.resolver.Get(ctx, entity.NotifyTransport(kind))
}

// initResolver builds a per-test registry+resolver over the shared DB, registers
// a uniquely named resolvable kind (so parallel tests do not share the single
// "resolvable" row), and returns the harness. Rows are dropped on cleanup.
func initResolver(t *testing.T) harness {
	t.Helper()

	kind := "resolvable-" + xuuid.NewString()

	kindRegistry, err := integrationsvc.NewRegistry(namedResolvable{name: kind})
	require.NoError(t, err)

	registry := integrationsvc.NewService(
		dbtx.NewTxManager(db),
		integrationstore.NewStore(db),
		datakeystore.NewStore(db),
		kindRegistry,
		keyring,
		secrets.NewAESCipher(),
		publishermock.New(t),
	)

	builders := transportresolver.Builders()
	builders[entity.NotifyTransport(kind)] = buildResolvable
	resolver := transportresolver.New(registry, builders)
	registry.SetOnChange(resolver.Invalidate)

	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM integration_settings WHERE kind = $1`, kind)
	})

	return harness{registry: registry, resolver: resolver, kind: kind}
}

func testActor() *entity.User {
	return &entity.User{ID: uuid.New(), Email: "admin@test.local", Name: "Admin"}
}

func testKEK() string {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 7)
	}
	return hex.EncodeToString(key)
}

func secretsJSON(t *testing.T, m map[string]string) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(m)
	require.NoError(t, err)
	return raw
}

func secretIntentsJSON(t *testing.T, m map[string]*string) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(m)
	require.NoError(t, err)
	return raw
}

// namedResolvable is a test kind whose builder produces an inspectable transport,
// so resolve/cache behavior can be exercised end to end. Its secret is "token";
// Parse returns the plaintext token as the settings. The name is per-test so
// parallel runs get distinct UNIQUE(kind) rows.
type namedResolvable struct{ name string }

func (r namedResolvable) Kind() string       { return r.name }
func (namedResolvable) SecretKeys() []string { return []string{"token"} }

func (namedResolvable) Parse(_ json.RawMessage, secretsIn map[string]string) (integrationkinds.Settings, error) {
	return resolvableSettings(secretsIn["token"]), nil
}
func (namedResolvable) Validate(integrationkinds.Settings) error { return nil }

// resolvableSettings is namedResolvable's parsed settings: just the plaintext token.
type resolvableSettings string

func (resolvableSettings) Kind() string { return "resolvable" }

// buildResolvable is the delivery half of the resolvable kind — registered into
// the resolver's builder map the same way production kinds are.
func buildResolvable(s integrationkinds.Settings) (notifytransport.Transport, error) {
	return resolvableTransport{token: string(s.(resolvableSettings))}, nil
}

type resolvableTransport struct{ token string }

func (resolvableTransport) TransportID() entity.NotifyTransport {
	return entity.NotifyTransport("resolvable")
}
func (resolvableTransport) Send(context.Context, string, entity.NotifyMessage) error { return nil }

// Token exposes the decrypted token the transport was built with, so a test can
// assert resolve wired the right (freshly-decrypted) plaintext into the client.
func (r resolvableTransport) Token() string { return r.token }

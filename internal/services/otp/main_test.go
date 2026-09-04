package otp_test

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/secrets"
	"github.com/ruko1202/maintmode/internal/services/otp"
	"github.com/ruko1202/maintmode/internal/storages/authcredentials"
	"github.com/ruko1202/maintmode/internal/storages/users"
	"github.com/ruko1202/maintmode/internal/utils/closer"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"
)

const testKEKURI = "local-kms://test"

// testKEKHex is the local KEK the suite seals under. Non-zero on purpose: the
// keyring rejects an all-zero key as an unset placeholder.
var testKEKHex = strings.Repeat("a7", 32)

var (
	db         *sqlx.DB
	credStore  *authcredentials.Store
	usersStore *users.Store
	txManager  *dbtx.TxManager
	cfg        *config.AppConfig
)

func TestMain(m *testing.M) {
	cfg = config.LoadAppConfig()
	db = testdbconnutils.NewDB(cfg)
	closer.Add(db.Close)

	credStore = authcredentials.NewStore(db)
	usersStore = users.NewStore(db)
	txManager = dbtx.NewTxManager(db)

	os.Exit(m.Run())
}

// newService wires the service over the real store and real crypto. Only the
// scheduler is a fake: it is the seam this suite inspects, standing in for the
// goque row a delivery task would become.
func newService(t *testing.T) (*otp.Service, *recordingScheduler) {
	t.Helper()

	sched := &recordingScheduler{}
	return newServiceWith(t, sched), sched
}

// newServiceWith wires the service over an arbitrary scheduler, so a test can
// use the real one and inspect what actually lands in goque_task.
func newServiceWith(t *testing.T, sched otp.TaskScheduler) *otp.Service {
	t.Helper()

	return otp.NewService(cfg, txManager, credStore, usersStore, testKeyring(t), secrets.NewAESCipher(), sched)
}

// failingKeyring makes WrapDEK fail, standing in for the KMS being unreachable
// -- the only network call inside the issuance transaction, and so the failure
// most likely to happen in production.
type failingKeyring struct{ err error }

func (k failingKeyring) WrapDEK([]byte) (wrapped []byte, kekID string, err error) {
	return nil, "", k.err
}

// failingCipher makes Encrypt fail.
type failingCipher struct{ err error }

func (c failingCipher) Encrypt(_, _, _ []byte) ([]byte, error) { return nil, c.err }

// newServiceWithCrypto wires the service over a chosen keyring and cipher, so a
// test can fail the sealing step and watch the transaction unwind.
func newServiceWithCrypto(t *testing.T, kr otp.Keyring, ciph otp.Cipher) *otp.Service {
	t.Helper()

	return otp.NewService(cfg, txManager, credStore, usersStore, kr, ciph, &recordingScheduler{})
}

func testKeyring(t *testing.T) *secrets.Keyring {
	t.Helper()

	kr, err := secrets.NewLocalKeyring(testKEKURI, map[string]string{testKEKURI: testKEKHex})
	require.NoError(t, err)

	return kr
}

// makeUser inserts a user to request codes for. The suite runs with -count 2
// against a shared database, so the address is randomized rather than derived
// from the test name -- a reused one would collide on the second pass.
func makeUser(ctx context.Context, t *testing.T) *entity.User {
	t.Helper()

	u, err := usersStore.Create(ctx, &entity.User{
		Email: uuid.NewString() + "@email.com",
		Name:  "otp test user",
		Roles: entity.DefaultRoles,
	})
	require.NoError(t, err)

	return u
}

// recordingScheduler captures enqueued tasks. Concurrency-safe because one test
// drives two issuances at once.
type recordingScheduler struct {
	mu    sync.Mutex
	tasks []scheduledTask
	err   error
}

type scheduledTask struct {
	taskType       string
	payload        any
	idempotencyKey string
}

func (s *recordingScheduler) Schedule(
	_ context.Context,
	taskType string,
	payload any,
	idempotencyKey string,
) (uuid.UUID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.err != nil {
		return uuid.Nil, s.err
	}

	s.tasks = append(s.tasks, scheduledTask{taskType, payload, idempotencyKey})
	return uuid.New(), nil
}

func (s *recordingScheduler) recorded() []scheduledTask {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]scheduledTask(nil), s.tasks...)
}

func (s *recordingScheduler) only(t *testing.T) entity.ProcessorTaskPayloadOTPEmail {
	t.Helper()

	tasks := s.recorded()
	require.Len(t, tasks, 1)
	require.Equal(t, entity.ProcessorTaskOTPEmailSend, tasks[0].taskType)

	payload, ok := tasks[0].payload.(entity.ProcessorTaskPayloadOTPEmail)
	require.True(t, ok, "unexpected payload type %T", tasks[0].payload)

	return payload
}

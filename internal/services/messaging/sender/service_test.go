package sender

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/ruko1202/goque"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	gwmsg "github.com/ruko1202/maintmode/internal/gateways/messengers"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

// captureMessenger implements gwmsg.Messenger and records every Send.
type captureMessenger struct {
	id      entity.MessengerID
	failErr error
	mu      sync.Mutex
	calls   []capturedCall
}

type capturedCall struct {
	Target string
	Msg    entity.Message
}

func (c *captureMessenger) MessengerID() entity.MessengerID { return c.id }

func (c *captureMessenger) Send(_ context.Context, target string, msg entity.Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, capturedCall{Target: target, Msg: msg})
	return c.failErr
}

func (c *captureMessenger) Calls() []capturedCall {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]capturedCall, len(c.calls))
	copy(out, c.calls)
	return out
}

// fakeQueue captures enqueued tasks and whether each carried a tx.
type fakeQueue struct {
	mu    sync.Mutex
	tasks []*goque.Task
	hadTx []bool
	err   error
}

func (q *fakeQueue) AsyncAddTaskToQueue(ctx context.Context, task *goque.Task) {
	_ = q.AddTaskToQueue(ctx, task)
}
func (q *fakeQueue) AddTaskToQueue(ctx context.Context, task *goque.Task) error {
	if q.err != nil {
		return q.err
	}
	_, hadTx := goque.TxFromContext(ctx)
	q.mu.Lock()
	defer q.mu.Unlock()
	q.tasks = append(q.tasks, task)
	q.hadTx = append(q.hadTx, hadTx)
	return nil
}
func (q *fakeQueue) GetTask(context.Context, uuid.UUID) (*goque.Task, error) { return nil, nil }
func (q *fakeQueue) GetTasks(context.Context, *goque.TaskFilter, int64) ([]*goque.Task, error) {
	return nil, nil
}
func (q *fakeQueue) ResetAttempts(context.Context, uuid.UUID) error { return nil }
func (q *fakeQueue) CancelTask(context.Context, uuid.UUID) error    { return nil }
func (q *fakeQueue) WaitAsyncEnqueues()                             {}

func (q *fakeQueue) Tasks() []*goque.Task {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]*goque.Task, len(q.tasks))
	copy(out, q.tasks)
	return out
}
func (q *fakeQueue) HadTx() []bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]bool, len(q.hadTx))
	copy(out, q.hadTx)
	return out
}

// buildService wires a Service around a MessengerRegistry built from
// the given messengers. UseStub stays false so the registry routes by
// MessengerID, not to the stub.
func buildService(t *testing.T, queue goque.TaskQueueManager, messengers ...gwmsg.Messenger) *Service {
	t.Helper()
	cfg := &config.AppConfig{Environment: config.LocalEnvironment}
	reg := gwmsg.NewMessengerRegistry(cfg, messengers...)
	return NewMessengerService(reg, queue)
}

func TestSend_DeliversToMessenger(t *testing.T) {
	t.Parallel()
	m := &captureMessenger{id: entity.MessengerSlack}
	svc := buildService(t, &fakeQueue{}, m)

	err := svc.Send(context.Background(), entity.MessengerSlack, "#x", entity.Message{Subject: "s", Body: "b"})
	require.NoError(t, err)
	require.Len(t, m.Calls(), 1)
	require.Equal(t, "#x", m.Calls()[0].Target)
	require.Equal(t, "s", m.Calls()[0].Msg.Subject)
}

func TestSend_PropagatesMessengerError(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("slack 503")
	m := &captureMessenger{id: entity.MessengerSlack, failErr: wantErr}
	svc := buildService(t, &fakeQueue{}, m)

	err := svc.Send(context.Background(), entity.MessengerSlack, "#x", entity.Message{Body: "b"})
	require.ErrorIs(t, err, wantErr)
}

func TestSend_UnknownMessengerReturnsError(t *testing.T) {
	t.Parallel()
	// Registry has only Slack; we send to Telegram.
	svc := buildService(t, &fakeQueue{}, &captureMessenger{id: entity.MessengerSlack})
	err := svc.Send(context.Background(), entity.MessengerTelegram, "42", entity.Message{})
	require.ErrorContains(t, err, "no transport")
}

func TestSend_RejectsCallInsideTx(t *testing.T) {
	t.Parallel()
	svc := buildService(t, &fakeQueue{}, &captureMessenger{id: entity.MessengerSlack})
	ctx := dbtx.WithTx(context.Background(), &sqlx.Tx{})
	err := svc.Send(ctx, entity.MessengerSlack, "#x", entity.Message{})
	require.Error(t, err, "Send must refuse to run inside a tx — caller should use SendAsync")
}

func TestSendAsync_EnqueuesWithoutDelivery(t *testing.T) {
	t.Parallel()
	q := &fakeQueue{}
	m := &captureMessenger{id: entity.MessengerSlack}
	svc := buildService(t, q, m)

	err := svc.SendAsync(context.Background(), entity.MessengerSlack, "#x", entity.Message{Body: "b"})
	require.NoError(t, err)
	require.Len(t, q.Tasks(), 1)
	require.Empty(t, m.Calls(), "async send must not call the messenger inline")
	require.WithinDuration(t, time.Now(), q.Tasks()[0].NextAttemptAt, 2*time.Second)
}

func TestSendDelayed_SetsNextAttemptAtInFuture(t *testing.T) {
	t.Parallel()
	q := &fakeQueue{}
	svc := buildService(t, q, &captureMessenger{id: entity.MessengerSlack})

	const delay = 30 * time.Minute
	before := time.Now()
	err := svc.SendDelayed(context.Background(), entity.MessengerSlack, "#x", entity.Message{Body: "b"}, delay)
	require.NoError(t, err)

	tasks := q.Tasks()
	require.Len(t, tasks, 1)
	require.WithinDuration(t, before.Add(delay), tasks[0].NextAttemptAt, 2*time.Second)
}

// TestSendAsync_OutboxEnrollsTxInGoque: ctx with maintmode *sqlx.Tx → goque
// receives a ctx carrying the same tx via goque.WithTx.
func TestSendAsync_OutboxEnrollsTxInGoque(t *testing.T) {
	t.Parallel()
	q := &fakeQueue{}
	svc := buildService(t, q, &captureMessenger{id: entity.MessengerSlack})

	ctx := dbtx.WithTx(context.Background(), &sqlx.Tx{})
	err := svc.SendAsync(ctx, entity.MessengerSlack, "#x", entity.Message{Body: "b"})
	require.NoError(t, err)

	require.Len(t, q.HadTx(), 1)
	require.True(t, q.HadTx()[0], "enqueue inside a tx ctx must enroll in the caller's tx via goque.WithTx")
}

func TestSendAsync_NoTx_NoOutbox(t *testing.T) {
	t.Parallel()
	q := &fakeQueue{}
	svc := buildService(t, q, &captureMessenger{id: entity.MessengerSlack})

	err := svc.SendAsync(context.Background(), entity.MessengerSlack, "#x", entity.Message{Body: "b"})
	require.NoError(t, err)

	require.Len(t, q.HadTx(), 1)
	require.False(t, q.HadTx()[0])
}

func TestSendAsync_PropagatesEnqueueError(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("pg down")
	q := &fakeQueue{err: wantErr}
	svc := buildService(t, q, &captureMessenger{id: entity.MessengerSlack})

	err := svc.SendAsync(context.Background(), entity.MessengerSlack, "#x", entity.Message{Body: "b"})
	require.ErrorIs(t, err, wantErr)
}

func TestSendAsync_IdempotencyKey_StableExternalID(t *testing.T) {
	t.Parallel()
	q := &fakeQueue{}
	svc := buildService(t, q, &captureMessenger{id: entity.MessengerSlack})

	const key = "stable-key-42"
	require.NoError(t, svc.SendAsync(context.Background(), entity.MessengerSlack, "#x", entity.Message{Body: "b"},
		WithIdempotencyKey(key)))
	require.NoError(t, svc.SendAsync(context.Background(), entity.MessengerSlack, "#x", entity.Message{Body: "b"},
		WithIdempotencyKey(key)))

	tasks := q.Tasks()
	require.Len(t, tasks, 2, "fakeQueue does not enforce uniqueness — goque's index does in real storage")
	require.Equal(t, tasks[0].ExternalID, tasks[1].ExternalID,
		"same idempotency key must produce identical external_id so goque dedupes")
}

func TestSendAsync_DefaultIdempotencyKey_IsUnique(t *testing.T) {
	t.Parallel()
	q := &fakeQueue{}
	svc := buildService(t, q, &captureMessenger{id: entity.MessengerSlack})

	require.NoError(t, svc.SendAsync(context.Background(), entity.MessengerSlack, "#x", entity.Message{Body: "b"}))
	require.NoError(t, svc.SendAsync(context.Background(), entity.MessengerSlack, "#x", entity.Message{Body: "b"}))

	tasks := q.Tasks()
	require.Len(t, tasks, 2)
	require.NotEqual(t, tasks[0].ExternalID, tasks[1].ExternalID,
		"without an explicit key, each enqueue must get a unique external_id")
}

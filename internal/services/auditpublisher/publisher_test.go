package auditpublisher

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/ruko1202/goque"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

// fakeEnqueuer records the task it was asked to enqueue, the ctx it saw (to
// assert the tx-join), and can simulate an enqueue failure.
type fakeEnqueuer struct {
	task   *goque.Task
	gotTx  bool
	err    error
	called int
}

func (f *fakeEnqueuer) AddTaskToQueue(ctx context.Context, task *goque.Task) error {
	f.called++
	_, f.gotTx = dbtx.TxFromContext(ctx)
	if f.err != nil {
		return f.err
	}
	f.task = task
	return nil
}

// fakeRenderer returns a fixed payload or an error.
type fakeRenderer struct {
	payload entity.ProcessorTaskPayloadAuditWrite
	err     error
}

func (f fakeRenderer) Render(audit.Action) (entity.ProcessorTaskPayloadAuditWrite, error) {
	return f.payload, f.err
}

func newPublisher(q taskEnqueuer, r renderer) *Publisher {
	return &Publisher{queue: q, renderer: r}
}

// loginAction is any supported action; rendering is faked, so its shape is moot.
func loginAction() audit.Action {
	return audit.LoginSuccess{User: &entity.User{ID: uuid.New(), Email: "a@example.com"}}
}

func TestPublish_EnqueuesTaskWithEventIDAsExternalID(t *testing.T) {
	eventID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	q := &fakeEnqueuer{}
	p := newPublisher(q, fakeRenderer{payload: entity.ProcessorTaskPayloadAuditWrite{EventID: eventID}})

	require.NoError(t, p.Publish(context.Background(), loginAction()))

	require.Equal(t, 1, q.called)
	require.NotNil(t, q.task)
	require.Equal(t, entity.ProcessorTaskAuditWrite, q.task.Type)
	// external_id is the per-event UUID — the dedup key goque enforces.
	require.Equal(t, eventID.String(), q.task.ExternalID)
}

func TestPublish_PropagatesRenderError(t *testing.T) {
	renderErr := errors.New("bad event")
	q := &fakeEnqueuer{}
	p := newPublisher(q, fakeRenderer{err: renderErr})

	err := p.Publish(context.Background(), loginAction())

	require.ErrorIs(t, err, renderErr)
	require.Zero(t, q.called, "a render failure must not enqueue anything")
}

func TestPublish_PropagatesEnqueueError(t *testing.T) {
	enqErr := errors.New("queue down")
	q := &fakeEnqueuer{err: enqErr}
	p := newPublisher(q, fakeRenderer{payload: entity.ProcessorTaskPayloadAuditWrite{EventID: uuid.New()}})

	err := p.Publish(context.Background(), loginAction())

	require.ErrorIs(t, err, enqErr)
}

func TestPublish_JoinsTxFromContext(t *testing.T) {
	q := &fakeEnqueuer{}
	p := newPublisher(q, fakeRenderer{payload: entity.ProcessorTaskPayloadAuditWrite{EventID: uuid.New()}})

	// A tx in ctx must be carried into the enqueue (transactional outbox), so the
	// insert participates in the caller's tx.
	ctx := dbtx.WithTx(context.Background(), &sqlx.Tx{})
	require.NoError(t, p.Publish(ctx, loginAction()))

	require.True(t, q.gotTx, "enqueue must run with the caller's tx attached to ctx")
}

func TestPublish_NoTxIsAutocommit(t *testing.T) {
	q := &fakeEnqueuer{}
	p := newPublisher(q, fakeRenderer{payload: entity.ProcessorTaskPayloadAuditWrite{EventID: uuid.New()}})

	require.NoError(t, p.Publish(context.Background(), loginAction()))

	require.False(t, q.gotTx, "without a tx in ctx the enqueue is a plain autocommit")
}

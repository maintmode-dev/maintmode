package auditprocessor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/ruko1202/goque"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

type fakeAuditWriter struct {
	calls []*entity.AuditEntry
	err   error
}

func (f *fakeAuditWriter) AddLog(_ context.Context, entry *entity.AuditEntry) error {
	f.calls = append(f.calls, entry)
	return f.err
}

func newTask(payload entity.ProcessorTaskPayloadAuditWrite) *goque.TypedTask[entity.ProcessorTaskPayloadAuditWrite] {
	return &goque.TypedTask[entity.ProcessorTaskPayloadAuditWrite]{
		Task:    &goque.Task{},
		Payload: payload,
	}
}

func samplePayload() entity.ProcessorTaskPayloadAuditWrite {
	return entity.ProcessorTaskPayloadAuditWrite{
		EventID:    uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		OccurredAt: time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC),
		Action:     entity.AuditActionLoginSuccess,
		Actor:      "user@example.com",
		EntityType: entity.AuditEntityTypeUser,
		EntityID:   "user-1",
		Details:    "login success",
	}
}

func TestProcessTask_WritesEntryWithOccurredAtAsCreatedAt(t *testing.T) {
	writer := &fakeAuditWriter{}
	p := newQueueProcessor(writer)

	payload := samplePayload()
	err := p.ProcessTask(context.Background(), newTask(payload))

	require.NoError(t, err)
	require.Len(t, writer.calls, 1)
	got := writer.calls[0]
	require.Equal(t, payload.EventID, got.ID)
	require.Equal(t, payload.EventID, got.EventID)
	// created_at must be the event time, not insert time — the ordering fix.
	require.Equal(t, payload.OccurredAt, got.CreatedAt)
	require.Equal(t, payload.Action, got.Action)
}

func TestProcessTask_ReturnsErrorOnWriteFailure(t *testing.T) {
	writeErr := errors.New("db down")
	writer := &fakeAuditWriter{err: writeErr}
	p := newQueueProcessor(writer)

	err := p.ProcessTask(context.Background(), newTask(samplePayload()))

	// The error is returned so goque retries (the metric increment is a
	// side-effect we exercise here for coverage; it degrades to a no-op meter
	// in tests).
	require.Error(t, err)
	require.ErrorIs(t, err, writeErr)
}

func TestProcessTask_RefusesToRunInsideTx(t *testing.T) {
	writer := &fakeAuditWriter{}
	p := newQueueProcessor(writer)

	ctx := dbtx.WithTx(context.Background(), &sqlx.Tx{})
	err := p.ProcessTask(ctx, newTask(samplePayload()))

	require.Error(t, err)
	require.Contains(t, err.Error(), "must not run inside a DB tx")
	require.Empty(t, writer.calls, "no write should happen when a tx is present")
}

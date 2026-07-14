package messagesender

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

// schedulerSpy captures the enqueued task; err (if set) is returned to prove
// propagation.
type schedulerSpy struct {
	err      error
	taskType string
	payload  any
	idemKey  string
}

func (s *schedulerSpy) Schedule(_ context.Context, taskType string, payload any, idempotencyKey string) (uuid.UUID, error) {
	s.taskType = taskType
	s.payload = payload
	s.idemKey = idempotencyKey
	return uuid.New(), s.err
}

// SendAsync builds the notify payload from its arguments and enqueues it under
// the caller's task type; a scheduler error propagates.
func TestSendAsync(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	msg := entity.NotifyMessage{Subject: "s", Body: "b", MessageMIME: entity.HTMLMessageMIME}

	t.Run("enqueues the built payload", func(t *testing.T) {
		t.Parallel()
		spy := &schedulerSpy{}
		svc := NewService(nil, spy)

		require.NoError(t, svc.SendAsync(ctx, entity.ProcessorTaskMessagingSend,
			entity.NotifyTransportSlack, "C1", msg, "idem-key"))

		require.Equal(t, entity.ProcessorTaskMessagingSend, spy.taskType)
		require.Equal(t, "idem-key", spy.idemKey)
		require.Equal(t, entity.ProcessorTaskPayloadEventNotify{
			TransportName: entity.NotifyTransportSlack,
			Target:        "C1",
			Subject:       "s",
			Body:          "b",
			MessageMIME:   entity.HTMLMessageMIME,
		}, spy.payload)
	})

	t.Run("scheduler error propagates", func(t *testing.T) {
		t.Parallel()
		spy := &schedulerSpy{err: errors.New("outbox down")}
		svc := NewService(nil, spy)

		require.Error(t, svc.SendAsync(ctx, entity.ProcessorTaskMessagingSend,
			entity.NotifyTransportSlack, "C1", msg, "idem-key"))
	})
}

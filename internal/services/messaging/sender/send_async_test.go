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

		root := &entity.MessageRef{MessageID: "1503435956.000247"}
		require.NoError(t, svc.SendAsync(ctx, entity.ProcessorTaskInvitationEmailSend,
			entity.NotifyTransportSlack, "C1", msg, root, "idem-key"))

		require.Equal(t, entity.ProcessorTaskInvitationEmailSend, spy.taskType)
		require.Equal(t, "idem-key", spy.idemKey)
		require.Equal(t, entity.ProcessorTaskPayloadEventNotify{
			TransportName: entity.NotifyTransportSlack,
			Target:        "C1",
			Subject:       "s",
			Body:          "b",
			MessageMIME:   entity.HTMLMessageMIME,
			// The enqueuing caller decides whether the message threads; the
			// payload is what carries that decision to the processor.
			ReplyTo: root,
		}, spy.payload)
	})

	t.Run("scheduler error propagates", func(t *testing.T) {
		t.Parallel()
		spy := &schedulerSpy{err: errors.New("outbox down")}
		svc := NewService(nil, spy)

		require.Error(t, svc.SendAsync(ctx, entity.ProcessorTaskInvitationEmailSend,
			entity.NotifyTransportSlack, "C1", msg, nil, "idem-key"))
	})
}

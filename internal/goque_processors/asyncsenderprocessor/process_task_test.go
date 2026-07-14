package asyncsenderprocessor

import (
	"context"
	"errors"
	"testing"

	"github.com/ruko1202/goque"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

// senderSpy captures the single Send call the thin processor makes and returns
// a canned error. The processor only unpacks the payload and delegates, so that
// delegation is all this test pins; the drop/retry policy is goque's.
type senderSpy struct {
	err error

	called    bool
	transport entity.NotifyTransport
	target    string
	msg       entity.NotifyMessage
}

func (s *senderSpy) Send(_ context.Context, trName entity.NotifyTransport, target string, msg entity.NotifyMessage) error {
	s.called = true
	s.transport = trName
	s.target = target
	s.msg = msg
	return s.err
}

func newTask() *goque.TypedTask[entity.ProcessorTaskPayloadEventNotify] {
	return &goque.TypedTask[entity.ProcessorTaskPayloadEventNotify]{
		Task: &goque.Task{},
		Payload: entity.ProcessorTaskPayloadEventNotify{
			TransportName: entity.NotifyTransportSlack,
			Target:        "C123",
			Subject:       "s",
			Body:          "b",
			MessageMIME:   entity.TextMessageMIME,
		},
	}
}

// The processor unpacks the payload verbatim into a single Send call.
func TestProcessTask_DelegatesPayloadToSend(t *testing.T) {
	t.Parallel()
	spy := &senderSpy{}
	p := newQueueProcessorProcessor(spy)

	require.NoError(t, p.ProcessTask(context.Background(), newTask()))
	require.True(t, spy.called)
	require.Equal(t, entity.NotifyTransportSlack, spy.transport)
	require.Equal(t, "C123", spy.target)
	require.Equal(t, entity.NotifyMessage{Subject: "s", Body: "b", MessageMIME: entity.TextMessageMIME}, spy.msg)
}

// A Send error is surfaced so goque applies its retry/drop policy — the
// processor adds no policy of its own.
func TestProcessTask_PropagatesSendError(t *testing.T) {
	t.Parallel()
	spy := &senderSpy{err: errors.New("boom")}
	p := newQueueProcessorProcessor(spy)

	require.Error(t, p.ProcessTask(context.Background(), newTask()))
}

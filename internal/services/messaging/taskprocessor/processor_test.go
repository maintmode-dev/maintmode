package taskprocessor

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/ruko1202/goque"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	gwmsg "github.com/ruko1202/maintmode/internal/gateways/messengers"
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

func newRegistry(messengers ...gwmsg.Messenger) *gwmsg.MessengerRegistry {
	cfg := &config.AppConfig{Environment: config.LocalEnvironment}
	return gwmsg.NewMessengerRegistry(cfg, messengers...)
}

// newTask builds the same goque.Task the sender would enqueue, so we
// can feed it straight to the processor.
func newTask(t *testing.T, payload entity.ProcessorTaskPayloadEventNotify) *goque.TypedTask[entity.ProcessorTaskPayloadEventNotify] {
	t.Helper()
	raw, err := goque.NewTaskWithPayloadAndExternalID(entity.ProcessorTaskMessagingSend, payload, uuid.NewString())
	require.NoError(t, err)

	var decoded entity.ProcessorTaskPayloadEventNotify
	require.NoError(t, json.Unmarshal([]byte(raw.Payload), &decoded))

	return &goque.TypedTask[entity.ProcessorTaskPayloadEventNotify]{
		Task:    raw,
		Payload: decoded,
	}
}

// (note: TypedTask embeds *Task as an anonymous field, so the literal
// above relies on the field being named Task at promotion level.)

func TestProcessTask_DispatchesByPayloadTransport(t *testing.T) {
	t.Parallel()
	slack := &captureMessenger{id: entity.MessengerSlack}
	telegram := &captureMessenger{id: entity.MessengerTelegram}
	p := newProcessor(newRegistry(slack, telegram))

	task := newTask(t, entity.ProcessorTaskPayloadEventNotify{
		TransportName: entity.MessengerTelegram,
		Target:        "42",
		Subject:       "s",
		Body:          "hi",
	})
	require.NoError(t, p.ProcessTask(context.Background(), task))

	require.Empty(t, slack.Calls(), "telegram payload must not call slack messenger")
	require.Len(t, telegram.Calls(), 1)
	require.Equal(t, "42", telegram.Calls()[0].Target)
	require.Equal(t, "hi", telegram.Calls()[0].Msg.Body)
}

func TestProcessTask_PropagatesMessengerError(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("slack 503")
	slack := &captureMessenger{id: entity.MessengerSlack, failErr: wantErr}
	p := newProcessor(newRegistry(slack))

	task := newTask(t, entity.ProcessorTaskPayloadEventNotify{
		TransportName: entity.MessengerSlack,
		Target:        "#x",
		Body:          "b",
	})
	err := p.ProcessTask(context.Background(), task)
	require.ErrorIs(t, err, wantErr,
		"processor must return messenger error so goque applies its retry policy")
}

func TestProcessTask_UnknownMessengerReturnsError(t *testing.T) {
	t.Parallel()
	// Registry has Telegram only; payload references Slack.
	p := newProcessor(newRegistry(&captureMessenger{id: entity.MessengerTelegram}))

	task := newTask(t, entity.ProcessorTaskPayloadEventNotify{
		TransportName: entity.MessengerSlack,
		Target:        "#x",
		Body:          "b",
	})
	err := p.ProcessTask(context.Background(), task)
	require.ErrorContains(t, err, "no transport")
}

func TestTaskPayload_RoundTripsThroughGoqueJSON(t *testing.T) {
	t.Parallel()
	in := entity.ProcessorTaskPayloadEventNotify{
		TransportName: entity.MessengerSlack,
		Target:        "#x",
		Subject:       "subject",
		Body:          "body",
	}
	task, err := goque.NewTaskWithPayloadAndExternalID(entity.ProcessorTaskMessagingSend, in, uuid.NewString())
	require.NoError(t, err)

	var out entity.ProcessorTaskPayloadEventNotify
	require.NoError(t, json.Unmarshal([]byte(task.Payload), &out))
	require.Equal(t, in, out)
}

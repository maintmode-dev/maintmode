package eventbus_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/eventbus"
	"github.com/ruko1202/maintmode/internal/eventbus/events"
)

const stubEventType events.EventType = "stub"

// stubEvent is a minimal event for exercising the bus.
type stubEvent struct{}

func (stubEvent) EventType() events.EventType { return stubEventType }

// fakeListener records events it handled and can simulate failure.
type fakeListener struct {
	err     error
	handled []eventbus.Event
}

func (f *fakeListener) SupportEvents() []events.EventType {
	return []events.EventType{stubEventType}
}

func (f *fakeListener) Handle(_ context.Context, e eventbus.Event) error {
	f.handled = append(f.handled, e)
	return f.err
}

func TestDispatcher_Dispatch(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	event := stubEvent{}

	t.Run("fans out to every listener subscribed to the event type", func(t *testing.T) {
		t.Parallel()

		a := &fakeListener{}
		b := &fakeListener{}

		err := eventbus.NewDispatcher(a, b).Dispatch(ctx, event)

		require.NoError(t, err)
		require.Equal(t, []eventbus.Event{event}, a.handled)
		require.Equal(t, []eventbus.Event{event}, b.handled)
	})

	t.Run("unknown event type returns ErrUnsupportedEvent", func(t *testing.T) {
		t.Parallel()

		err := eventbus.NewDispatcher().Dispatch(ctx, event)

		require.ErrorIs(t, err, apperr.ErrUnsupportedEvent)
	})

	t.Run("best-effort: a failing listener does not stop the rest", func(t *testing.T) {
		t.Parallel()

		failing := &fakeListener{err: errors.New("boom")}
		after := &fakeListener{}

		// failing first → after must still receive the event; the joined error
		// is returned, not panicked.
		err := eventbus.NewDispatcher(failing, after).Dispatch(ctx, event)

		require.Error(t, err)
		require.Equal(t, []eventbus.Event{event}, failing.handled)
		require.Equal(t, []eventbus.Event{event}, after.handled)
	})
}

func TestDispatcher_AsyncDispatch_DrainsOnStop(t *testing.T) {
	t.Parallel()

	l := &fakeListener{}
	d := eventbus.NewDispatcher(l)

	d.AsyncDispatch(context.Background(), stubEvent{})

	// Stop must wait for the in-flight async handler to finish.
	require.NoError(t, d.Stop(context.Background()))
	require.Equal(t, []eventbus.Event{stubEvent{}}, l.handled)
}

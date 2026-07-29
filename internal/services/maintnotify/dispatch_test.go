package maintnotify

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/entity"
)

func target(transport entity.NotifyTransport, channel string) *entity.NotifyTarget {
	return &entity.NotifyTarget{
		ID:                 uuid.New(),
		ChannelID:          uuid.New(),
		Transport:          transport,
		TransportChannelID: channel,
	}
}

func ptr(s string) *string { return &s }

// errRenderBoom is the injected render failure used to prove that no target is
// contacted when any transport fails to render.
var errRenderBoom = errors.New("render boom")

// sentMessage records one delivery so tests can assert on the body that actually
// reached each transport.
type sentMessage struct {
	transport entity.NotifyTransport
	body      string
}

// recorder collects deliveries from the mock sender. Dispatch is sequential, but
// the mutex keeps the recorder safe if that ever changes.
type recorder struct {
	mu   sync.Mutex
	sent []sentMessage
}

func (r *recorder) record(transport entity.NotifyTransport, msg entity.NotifyMessage) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.sent = append(r.sent, sentMessage{transport: transport, body: msg.Body})
}

func (r *recorder) all() []sentMessage {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]sentMessage(nil), r.sent...)
}

// failOnTransport renders everything except one transport, which it fails. It
// exists because no real input can make render succeed for one transport and
// fail for another — failure depends on the event kind and the frontend URL,
// both transport-independent — yet that is precisely the ordering the invariant
// under test must survive.
type failOnTransport struct {
	inner  EventRenderer
	failOn entity.NotifyTransport
	err    error
}

func (f *failOnTransport) Render(
	ctx context.Context,
	transport entity.NotifyTransport,
	evt entity.NotifyEvent,
) (entity.NotifyMessage, error) {
	if transport == f.failOn {
		return entity.NotifyMessage{}, f.err
	}

	return f.inner.Render(ctx, transport, evt)
}

// TestDispatchRenderFailureSendsNothing is the load-bearing test of this change.
//
// dispatchSync has no idempotency key, and an error returned to the reminder
// processor makes goque retry the whole task. So if slack were rendered and sent
// before telegram failed to render, the retry would deliver to slack a second
// time. Rendering every transport before the first send is the only structural
// defense, and this test is what holds it in place: it fails the moment someone
// "optimizes" dispatch into rendering lazily inside the send loop.
func TestDispatchRenderFailureSendsNothing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	n, mocks := initNotifier(t)
	expectNoMentions(mocks)

	// Slack is listed first and renders fine; telegram fails. A lazy
	// implementation would therefore have already delivered to slack.
	n.renderer = &failOnTransport{
		inner:  n.renderer,
		failOn: entity.NotifyTransportTelegram,
		err:    errRenderBoom,
	}

	mocks.notifyTarget.EXPECT().
		ListByMaint(gomock.Any(), gomock.Any()).
		Return([]*entity.NotifyTarget{
			target(entity.NotifyTransportSlack, "#ops"),
			target(entity.NotifyTransportTelegram, "-100500"),
		}, nil)

	// No Send expectation at all: the gomock controller fails the test on any
	// call, which is exactly the assertion "zero sends".
	err := n.dispatchSync(ctx, entity.NotifyEvent{
		Kind:       entity.NotifyEventMaintStarted,
		MaintID:    uuid.New(),
		MaintTitle: "DB upgrade",
	})

	require.ErrorIs(t, err, errRenderBoom)
}

// TestDispatchUnknownKindSendsNothing covers the render failure reachable from
// real input — an event kind with no template — which fails for every transport.
func TestDispatchUnknownKindSendsNothing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	n, mocks := initNotifier(t)
	expectNoMentions(mocks)

	mocks.notifyTarget.EXPECT().
		ListByMaint(gomock.Any(), gomock.Any()).
		Return([]*entity.NotifyTarget{target(entity.NotifyTransportSlack, "#ops")}, nil)

	err := n.dispatchSync(ctx, entity.NotifyEvent{
		Kind:    entity.NotifyEventKind("maint.nonexistent"),
		MaintID: uuid.New(),
	})

	require.Error(t, err)
}

// TestDispatchRendersOncePerTransport pins that rendering is keyed by transport,
// not by target: four targets across two transports must produce two renders.
func TestDispatchRendersOncePerTransport(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	n, mocks := initNotifier(t)
	expectNoMentions(mocks)
	ownerID := uuid.New()

	mocks.notifyTarget.EXPECT().
		ListByMaint(gomock.Any(), gomock.Any()).
		Return([]*entity.NotifyTarget{
			target(entity.NotifyTransportSlack, "#ops"),
			target(entity.NotifyTransportSlack, "#eng"),
			target(entity.NotifyTransportTelegram, "-100500"),
			target(entity.NotifyTransportTelegram, "-100600"),
		}, nil)

	// Resolved exactly once for the whole dispatch, ahead of the send loop.
	mocks.ownerResolver.EXPECT().
		ResolveOwner(gomock.Any(), ownerID).
		Return(&entity.UserMention{
			Name:        "Ruslan Kosykh",
			TelegramTag: ptr("@ruslan_tg"),
			SlackTag:    ptr("ruslan.slack"),
		}).
		Times(1)

	rec := &recorder{}
	mocks.sender.EXPECT().
		Send(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, tr entity.NotifyTransport, _ string, msg entity.NotifyMessage) error {
			rec.record(tr, msg)

			return nil
		}).
		Times(4)

	err := n.dispatchSync(ctx, entity.NotifyEvent{
		Kind:            entity.NotifyEventMaintStarted,
		MaintID:         uuid.New(),
		MaintTitle:      "DB upgrade",
		CreatedByUserID: ownerID,
	})
	require.NoError(t, err)

	// Both targets on a transport receive the identical body — one render each,
	// fanned out — and the two transports differ by the handle they carry.
	sent := rec.all()
	require.Len(t, sent, 4)

	bodies := map[entity.NotifyTransport]map[string]struct{}{}
	for _, s := range sent {
		if bodies[s.transport] == nil {
			bodies[s.transport] = map[string]struct{}{}
		}

		bodies[s.transport][s.body] = struct{}{}
	}

	assert.Len(t, bodies[entity.NotifyTransportSlack], 1, "slack targets must share one rendered body")
	assert.Len(t, bodies[entity.NotifyTransportTelegram], 1, "telegram targets must share one rendered body")

	for body := range bodies[entity.NotifyTransportSlack] {
		assert.Contains(t, body, "Owner: ruslan.slack")
		assert.NotContains(t, body, "@ruslan_tg")
	}

	for body := range bodies[entity.NotifyTransportTelegram] {
		assert.Contains(t, body, "Owner: @ruslan_tg")
		assert.NotContains(t, body, "ruslan.slack")
	}
}

// TestDispatchStepEventSkipsOwnerResolve is the regression guard: step paths
// leave CreatedByUserID zero, so they must pass the resolver by without a call
// and render exactly as before.
func TestDispatchStepEventSkipsOwnerResolve(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	n, mocks := initNotifier(t)
	expectNoMentions(mocks)

	mocks.notifyTarget.EXPECT().
		ListByMaint(gomock.Any(), gomock.Any()).
		Return([]*entity.NotifyTarget{target(entity.NotifyTransportSlack, "#ops")}, nil)

	// No ResolveOwner expectation: any call fails the test.
	rec := &recorder{}
	mocks.sender.EXPECT().
		Send(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, tr entity.NotifyTransport, _ string, msg entity.NotifyMessage) error {
			rec.record(tr, msg)
			assert.Equal(t, entity.TextMessageMIME, msg.MessageMIME)

			return nil
		})

	n.NotifyStepLifecycle(ctx,
		entity.NotifyEventStepStarted,
		&entity.Maintenance{ID: uuid.New(), Title: "DB upgrade", CreatedByUserID: uuid.New()},
		&entity.MaintenanceStep{ID: uuid.New(), Order: 2, Description: "Drain connections"},
	)

	sent := rec.all()
	require.Len(t, sent, 1)
	assert.NotContains(t, sent[0].body, "Owner:")
	assert.True(t, strings.HasPrefix(sent[0].body, "Step started"))
}

// TestDispatchBlockedOwner checks the whole path for a blocked owner: the label
// is visible in the body and no handle is pinged.
func TestDispatchBlockedOwner(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	n, mocks := initNotifier(t)
	expectNoMentions(mocks)
	ownerID := uuid.New()

	mocks.notifyTarget.EXPECT().
		ListByMaint(gomock.Any(), gomock.Any()).
		Return([]*entity.NotifyTarget{target(entity.NotifyTransportSlack, "#ops")}, nil)

	// This is what usersummary.ResolveOwner returns for a blocked user: the
	// labeled name, both handles withheld.
	mocks.ownerResolver.EXPECT().
		ResolveOwner(gomock.Any(), ownerID).
		Return(&entity.UserMention{Name: "Ruslan Kosykh [blocked]"})

	rec := &recorder{}
	mocks.sender.EXPECT().
		Send(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, tr entity.NotifyTransport, _ string, msg entity.NotifyMessage) error {
			rec.record(tr, msg)

			return nil
		})

	n.NotifyMaintLifecycle(ctx, entity.NotifyEventMaintStarted, &entity.Maintenance{
		ID:              uuid.New(),
		Title:           "DB upgrade",
		CreatedByUserID: ownerID,
	})

	sent := rec.all()
	require.Len(t, sent, 1)
	assert.Contains(t, sent[0].body, "Owner: Ruslan Kosykh [blocked]")
}

// TestDispatchReminderCarriesOwnerMention covers the reminder entry point, whose
// owner threading is separate from the lifecycle one.
func TestDispatchReminderCarriesOwnerMention(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	n, mocks := initNotifier(t)
	expectNoMentions(mocks)
	ownerID := uuid.New()

	mocks.notifyTarget.EXPECT().
		ListByMaint(gomock.Any(), gomock.Any()).
		Return([]*entity.NotifyTarget{target(entity.NotifyTransportTelegram, "-100500")}, nil)

	mocks.ownerResolver.EXPECT().
		ResolveOwner(gomock.Any(), ownerID).
		Return(&entity.UserMention{Name: "Ruslan Kosykh", TelegramTag: ptr("@ruslan_tg")})

	rec := &recorder{}
	mocks.sender.EXPECT().
		Send(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, tr entity.NotifyTransport, _ string, msg entity.NotifyMessage) error {
			rec.record(tr, msg)

			return nil
		})

	err := n.NotifyMaintReminder(ctx, &entity.Maintenance{
		ID:              uuid.New(),
		Title:           "DB upgrade",
		CreatedByUserID: ownerID,
	})
	require.NoError(t, err)

	sent := rec.all()
	require.Len(t, sent, 1)
	assert.Contains(t, sent[0].body, "Owner: @ruslan_tg")
}

// TestDispatchTargetWithoutTransportStillDelivers pins the fallback branch: a
// target carrying no transport must not break delivery, and must not collapse
// distinct transports into a shared rendered body.
func TestDispatchTargetWithoutTransportStillDelivers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	n, mocks := initNotifier(t)
	expectNoMentions(mocks)
	ownerID := uuid.New()

	mocks.notifyTarget.EXPECT().
		ListByMaint(gomock.Any(), gomock.Any()).
		Return([]*entity.NotifyTarget{
			target("", "#unknown"),
			target(entity.NotifyTransportSlack, "#ops"),
		}, nil)

	mocks.ownerResolver.EXPECT().
		ResolveOwner(gomock.Any(), ownerID).
		Return(&entity.UserMention{Name: "Ruslan Kosykh", SlackTag: ptr("ruslan.slack")})

	rec := &recorder{}
	mocks.sender.EXPECT().
		Send(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, tr entity.NotifyTransport, _ string, msg entity.NotifyMessage) error {
			rec.record(tr, msg)

			return nil
		}).
		Times(2)

	n.NotifyMaintLifecycle(ctx, entity.NotifyEventMaintStarted, &entity.Maintenance{
		ID:              uuid.New(),
		Title:           "DB upgrade",
		CreatedByUserID: ownerID,
	})

	sent := rec.all()
	require.Len(t, sent, 2)

	byTransport := map[entity.NotifyTransport]string{}
	for _, s := range sent {
		byTransport[s.transport] = s.body
	}

	// The transportless target falls back to the display name; the slack one
	// keeps its handle. Two distinct bodies prove they were not merged.
	assert.Contains(t, byTransport[""], "Owner: Ruslan Kosykh")
	assert.Contains(t, byTransport[entity.NotifyTransportSlack], "Owner: ruslan.slack")
}

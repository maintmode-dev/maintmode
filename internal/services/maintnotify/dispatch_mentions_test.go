package maintnotify

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/entity"
)

// Every assertion in this file reads the body the SENDER recorded, never
// event.Mentions. That is the whole point of the file: an earlier design of this
// feature would have persisted mentions, never delivered them to the renderer,
// and passed a test suite that asserted on the event struct. Only the delivered
// text proves the chain is connected end to end.

// recordSends wires the sender mock to a recorder and returns it.
func recordSends(mocks *serviceMocks, times int) *recorder {
	rec := &recorder{}
	mocks.sender.EXPECT().
		Send(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, tr entity.NotifyTransport, _ string, msg entity.NotifyMessage) error {
			rec.record(tr, msg)

			return nil
		}).
		Times(times)

	return rec
}

// TestDispatchLifecycleDeliversMentions is the primary anti-no-op assertion for
// the lifecycle path: two mentioned ids must both appear in the sent body.
func TestDispatchLifecycleDeliversMentions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	n, mocks := initNotifier(t)
	ownerID, maintID := uuid.New(), uuid.New()
	alice, bob := uuid.New(), uuid.New()

	mocks.notifyTarget.EXPECT().
		ListByMaint(gomock.Any(), gomock.Any()).
		Return([]*entity.NotifyTarget{target(entity.NotifyTransportSlack, "#ops")}, nil)

	mocks.ownerResolver.EXPECT().
		ResolveOwner(gomock.Any(), ownerID).
		Return(&entity.UserMention{Name: "Ruslan Kosykh", SlackTag: ptr("ruslan.slack")})

	mocks.mentions.EXPECT().
		GetMaintMentions(gomock.Any(), maintID).
		Return([]uuid.UUID{alice, bob}, nil).
		Times(1)

	mocks.mentionResolver.EXPECT().
		ResolveMentions(gomock.Any(), []uuid.UUID{alice, bob}).
		Return([]*entity.UserMention{
			{Name: "Alice", SlackTag: ptr("alice.slack")},
			{Name: "Bob"},
		}).
		Times(1)

	rec := recordSends(mocks, 1)

	n.NotifyMaintLifecycle(ctx, entity.NotifyEventMaintStarted, &entity.Maintenance{
		ID:              maintID,
		Title:           "DB upgrade",
		CreatedByUserID: ownerID,
	})

	sent := rec.all()
	require.Len(t, sent, 1)
	assert.Contains(t, sent[0].body, "Mentions: alice.slack, Bob")
	assert.Contains(t, sent[0].body, "Owner: ruslan.slack")
}

// TestDispatchReminderDeliversMentionsWithoutCallerHydration is THE no-op killer.
//
// The maintenance is constructed with Mentions deliberately left empty, exactly
// as the reminder processor produces it: it reads from the raw store, which
// hydrates no child collections, and NotifyMaintReminder then flattens the
// maintenance to five scalars. If the load lived anywhere but dispatch, this body
// would carry no mention line and every other test in the suite would still pass.
func TestDispatchReminderDeliversMentionsWithoutCallerHydration(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	n, mocks := initNotifier(t)
	ownerID, maintID := uuid.New(), uuid.New()
	alice := uuid.New()

	mocks.notifyTarget.EXPECT().
		ListByMaint(gomock.Any(), gomock.Any()).
		Return([]*entity.NotifyTarget{target(entity.NotifyTransportTelegram, "-100500")}, nil)

	mocks.ownerResolver.EXPECT().
		ResolveOwner(gomock.Any(), ownerID).
		Return(&entity.UserMention{Name: "Ruslan Kosykh", TelegramTag: ptr("@ruslan_tg")})

	mocks.mentions.EXPECT().
		GetMaintMentions(gomock.Any(), maintID).
		Return([]uuid.UUID{alice}, nil).
		Times(1)

	mocks.mentionResolver.EXPECT().
		ResolveMentions(gomock.Any(), []uuid.UUID{alice}).
		Return([]*entity.UserMention{{Name: "Alice", TelegramTag: ptr("@alice_tg")}}).
		Times(1)

	rec := recordSends(mocks, 1)

	maint := &entity.Maintenance{
		ID:              maintID,
		Title:           "DB upgrade",
		CreatedByUserID: ownerID,
	}
	// The load is dispatch's job, not the caller's.
	require.Empty(t, maint.Mentions)

	require.NoError(t, n.NotifyMaintReminder(ctx, maint))

	sent := rec.all()
	require.Len(t, sent, 1)
	assert.Contains(t, sent[0].body, "Mentions: @alice_tg")
}

// TestDispatchOwnerIsNotMentionedTwice pins that the owner is filtered by id
// before the resolve — entity.UserMention carries no ID, so no later layer could
// deduplicate them.
func TestDispatchOwnerIsNotMentionedTwice(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	n, mocks := initNotifier(t)
	ownerID, maintID := uuid.New(), uuid.New()
	alice := uuid.New()

	mocks.notifyTarget.EXPECT().
		ListByMaint(gomock.Any(), gomock.Any()).
		Return([]*entity.NotifyTarget{target(entity.NotifyTransportSlack, "#ops")}, nil)

	mocks.ownerResolver.EXPECT().
		ResolveOwner(gomock.Any(), ownerID).
		Return(&entity.UserMention{Name: "Ruslan Kosykh", SlackTag: ptr("ruslan.slack")})

	// The owner is in the stored mention list...
	mocks.mentions.EXPECT().
		GetMaintMentions(gomock.Any(), maintID).
		Return([]uuid.UUID{alice, ownerID}, nil).
		Times(1)

	// ...but the resolver must receive it WITHOUT them.
	mocks.mentionResolver.EXPECT().
		ResolveMentions(gomock.Any(), []uuid.UUID{alice}).
		Return([]*entity.UserMention{{Name: "Alice", SlackTag: ptr("alice.slack")}}).
		Times(1)

	rec := recordSends(mocks, 1)

	n.NotifyMaintLifecycle(ctx, entity.NotifyEventMaintStarted, &entity.Maintenance{
		ID:              maintID,
		Title:           "DB upgrade",
		CreatedByUserID: ownerID,
	})

	sent := rec.all()
	require.Len(t, sent, 1)
	assert.Equal(t, 1, countOccurrences(sent[0].body, "ruslan.slack"),
		"the owner must appear exactly once, on the Owner line")
	assert.Contains(t, sent[0].body, "Mentions: alice.slack")
}

// TestDispatchOwnerOnlyMentionSkipsResolve covers the list that empties out after
// the owner filter: there is nobody left, so no resolve call is worth making.
func TestDispatchOwnerOnlyMentionSkipsResolve(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	n, mocks := initNotifier(t)
	ownerID, maintID := uuid.New(), uuid.New()

	mocks.notifyTarget.EXPECT().
		ListByMaint(gomock.Any(), gomock.Any()).
		Return([]*entity.NotifyTarget{target(entity.NotifyTransportSlack, "#ops")}, nil)

	mocks.ownerResolver.EXPECT().
		ResolveOwner(gomock.Any(), ownerID).
		Return(&entity.UserMention{Name: "Ruslan Kosykh", SlackTag: ptr("ruslan.slack")})

	mocks.mentions.EXPECT().
		GetMaintMentions(gomock.Any(), maintID).
		Return([]uuid.UUID{ownerID}, nil).
		Times(1)

	mocks.mentionResolver.EXPECT().ResolveMentions(gomock.Any(), gomock.Any()).Times(0)

	rec := recordSends(mocks, 1)

	n.NotifyMaintLifecycle(ctx, entity.NotifyEventMaintStarted, &entity.Maintenance{
		ID:              maintID,
		Title:           "DB upgrade",
		CreatedByUserID: ownerID,
	})

	sent := rec.all()
	require.Len(t, sent, 1)
	assert.NotContains(t, sent[0].body, "Mentions")
}

// TestDispatchMentionsLoadFailureStillDelivers is the duplicate-delivery guard.
// dispatchSync has no idempotency key, so an error escaping here would become a
// goque retry that re-sends to every target. Losing the decoration is the correct
// trade.
func TestDispatchMentionsLoadFailureStillDelivers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	n, mocks := initNotifier(t)
	ownerID, maintID := uuid.New(), uuid.New()

	mocks.notifyTarget.EXPECT().
		ListByMaint(gomock.Any(), gomock.Any()).
		Return([]*entity.NotifyTarget{
			target(entity.NotifyTransportSlack, "#ops"),
			target(entity.NotifyTransportTelegram, "-100500"),
		}, nil)

	mocks.ownerResolver.EXPECT().
		ResolveOwner(gomock.Any(), ownerID).
		Return(&entity.UserMention{Name: "Ruslan Kosykh", SlackTag: ptr("ruslan.slack")})

	mocks.mentions.EXPECT().
		GetMaintMentions(gomock.Any(), maintID).
		Return(nil, errors.New("mentions table unreachable")).
		Times(1)

	mocks.mentionResolver.EXPECT().ResolveMentions(gomock.Any(), gomock.Any()).Times(0)

	rec := recordSends(mocks, 2)

	err := n.dispatchSync(ctx, entity.NotifyEvent{
		Kind:            entity.NotifyEventMaintStarted,
		MaintID:         maintID,
		MaintTitle:      "DB upgrade",
		CreatedByUserID: ownerID,
	})
	require.NoError(t, err, "a mentions load failure must never reach the caller")

	sent := rec.all()
	require.Len(t, sent, 2, "every target still receives its notification")
	for _, s := range sent {
		assert.NotContains(t, s.body, "Mentions")
		assert.Contains(t, s.body, "Maintenance started: DB upgrade")
	}

	// Swallowing the error is correct, but it makes the drop invisible: the
	// counter is the ONLY remaining signal that a mention list failed to load,
}

// TestDispatchResolvesMentionsOncePerEvent pins the batching contract across a
// two-transport fan-out: the resolve is per event, not per transport or target.
func TestDispatchResolvesMentionsOncePerEvent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	n, mocks := initNotifier(t)
	ownerID, maintID := uuid.New(), uuid.New()
	alice := uuid.New()

	mocks.notifyTarget.EXPECT().
		ListByMaint(gomock.Any(), gomock.Any()).
		Return([]*entity.NotifyTarget{
			target(entity.NotifyTransportSlack, "#ops"),
			target(entity.NotifyTransportSlack, "#eng"),
			target(entity.NotifyTransportTelegram, "-100500"),
		}, nil)

	mocks.ownerResolver.EXPECT().
		ResolveOwner(gomock.Any(), ownerID).
		Return(&entity.UserMention{Name: "Ruslan Kosykh"}).
		Times(1)

	mocks.mentions.EXPECT().
		GetMaintMentions(gomock.Any(), maintID).
		Return([]uuid.UUID{alice}, nil).
		Times(1)

	mocks.mentionResolver.EXPECT().
		ResolveMentions(gomock.Any(), []uuid.UUID{alice}).
		Return([]*entity.UserMention{
			{Name: "Alice", SlackTag: ptr("alice.slack"), TelegramTag: ptr("@alice_tg")},
		}).
		Times(1)

	rec := recordSends(mocks, 3)

	n.NotifyMaintLifecycle(ctx, entity.NotifyEventMaintStarted, &entity.Maintenance{
		ID:              maintID,
		Title:           "DB upgrade",
		CreatedByUserID: ownerID,
	})

	// One resolve, yet each transport still renders its own handle.
	byTransport := map[entity.NotifyTransport]string{}
	for _, s := range rec.all() {
		byTransport[s.transport] = s.body
	}

	assert.Contains(t, byTransport[entity.NotifyTransportSlack], "Mentions: alice.slack")
	assert.Contains(t, byTransport[entity.NotifyTransportTelegram], "Mentions: @alice_tg")
}

// TestDispatchStepEventSkipsMentionsLoad pins that step notifications never pay
// for the query — a step notification fires per step, so this is the difference
// between one lookup per maintenance and one per step.
func TestDispatchStepEventSkipsMentionsLoad(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	n, mocks := initNotifier(t)

	mocks.notifyTarget.EXPECT().
		ListByMaint(gomock.Any(), gomock.Any()).
		Return([]*entity.NotifyTarget{target(entity.NotifyTransportSlack, "#ops")}, nil)

	mocks.mentions.EXPECT().GetMaintMentions(gomock.Any(), gomock.Any()).Times(0)
	mocks.mentionResolver.EXPECT().ResolveMentions(gomock.Any(), gomock.Any()).Times(0)

	rec := recordSends(mocks, 1)

	n.NotifyStepLifecycle(ctx,
		entity.NotifyEventStepStarted,
		&entity.Maintenance{ID: uuid.New(), Title: "DB upgrade", CreatedByUserID: uuid.New()},
		&entity.MaintenanceStep{ID: uuid.New(), Order: 2, Description: "Drain connections"},
	)

	sent := rec.all()
	require.Len(t, sent, 1)
	assert.NotContains(t, sent[0].body, "Mentions")
}

// TestDispatchNoMentionsRendersTodaysBody is acceptance criterion 7 at the
// dispatch level: a maintenance without mentions must produce the body it
// produces today, and must not call the resolver.
func TestDispatchNoMentionsRendersTodaysBody(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	n, mocks := initNotifier(t)
	ownerID, maintID := uuid.New(), uuid.New()

	mocks.notifyTarget.EXPECT().
		ListByMaint(gomock.Any(), gomock.Any()).
		Return([]*entity.NotifyTarget{target(entity.NotifyTransportSlack, "#ops")}, nil)

	mocks.ownerResolver.EXPECT().
		ResolveOwner(gomock.Any(), ownerID).
		Return(&entity.UserMention{Name: "Ruslan Kosykh", SlackTag: ptr("ruslan.slack")})

	mocks.mentions.EXPECT().
		GetMaintMentions(gomock.Any(), maintID).
		Return(nil, nil).
		Times(1)

	mocks.mentionResolver.EXPECT().ResolveMentions(gomock.Any(), gomock.Any()).Times(0)

	rec := recordSends(mocks, 1)

	n.NotifyMaintLifecycle(ctx, entity.NotifyEventMaintStarted, &entity.Maintenance{
		ID:              maintID,
		Title:           "DB upgrade",
		CreatedByUserID: ownerID,
	})

	sent := rec.all()
	require.Len(t, sent, 1)
	assert.NotContains(t, sent[0].body, "Mentions")
	assert.Contains(t, sent[0].body, "Owner: ruslan.slack")
}

func countOccurrences(haystack, needle string) int {
	count := 0
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			count++
		}
	}

	return count
}

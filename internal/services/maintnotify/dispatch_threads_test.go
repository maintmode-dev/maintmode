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

// threadedTarget builds a target whose root was delivered to this channel's
// current address — the state in which threading is expected to work.
//
// RootChannel, not the canonical chat id the API answered with, is what the
// guard compares: the two are written by different systems and in production
// never match for a channel addressed as "#name" or "@name".
func threadedTarget(transport entity.NotifyTransport, channel, rootID string) *entity.NotifyTarget {
	tgt := target(transport, channel)
	tgt.RootRef = &entity.MessageRef{MessageID: rootID}
	tgt.RootChannel = channel

	return tgt
}

// recordThreadSends wires the sender to a recorder that also returns a ref, so
// maint.started deliveries produce something worth persisting.
func recordThreadSends(mocks *serviceMocks, rec *recorder, res entity.SendResult, times int) {
	mocks.sender.EXPECT().
		Send(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			tr entity.NotifyTransport,
			channel string,
			msg entity.NotifyMessage,
			replyTo *entity.MessageRef,
		) (entity.SendResult, error) {
			rec.record(tr, channel, msg, replyTo)

			return res, nil
		}).
		Times(times)
}

// maint.started IS the root, so it goes out top-level and its ref is persisted
// for every channel — each keyed by its own (maintenance, channel) pair.
func TestDispatchStartedWritesRootPerTarget(t *testing.T) {
	ctx := context.Background()

	n, mocks := initNotifier(t)
	expectNoMentions(mocks)
	maintID := uuid.New()

	slackTarget := target(entity.NotifyTransportSlack, "C123")
	tgTarget := target(entity.NotifyTransportTelegram, "-100500")

	mocks.notifyTarget.EXPECT().
		ListByMaint(gomock.Any(), maintID).
		Return([]*entity.NotifyTarget{slackTarget, tgTarget}, nil)

	rec := &recorder{}
	ref := entity.MessageRef{MessageID: "1503435956.000247"}
	recordThreadSends(mocks, rec, entity.SendResult{Ref: &ref}, 2)

	// The reference is persisted from the API RESPONSE, while the address is the
	// catalog one the message was delivered to — the two are stored separately
	// so the re-point guard has like-for-like values to compare later.
	mocks.notifyTarget.EXPECT().
		SetRootRef(gomock.Any(), maintID, slackTarget.ChannelID, ref.MessageID, "C123").Return(nil)
	mocks.notifyTarget.EXPECT().
		SetRootRef(gomock.Any(), maintID, tgTarget.ChannelID, ref.MessageID, "-100500").Return(nil)

	require.NoError(t, n.dispatchSync(ctx, entity.NotifyEvent{
		Kind:       entity.NotifyEventMaintStarted,
		MaintID:    maintID,
		MaintTitle: "DB upgrade",
	}))

	for _, sent := range rec.all() {
		assert.Nil(t, sent.replyTo, "maint.started is the root and must go out top-level")
	}
}

// The vertical slice: after a start recorded a root, the next lifecycle event
// replies to exactly that root — and two channels never cross threads.
func TestDispatchRepliesToEachChannelsOwnRoot(t *testing.T) {
	ctx := context.Background()

	n, mocks := initNotifier(t)
	expectNoMentions(mocks)
	maintID := uuid.New()

	slackTarget := threadedTarget(entity.NotifyTransportSlack, "C123", "1503435956.000247")
	tgTarget := threadedTarget(entity.NotifyTransportTelegram, "-100500", "41")

	mocks.notifyTarget.EXPECT().
		ListByMaint(gomock.Any(), maintID).
		Return([]*entity.NotifyTarget{slackTarget, tgTarget}, nil)

	rec := &recorder{}
	recordThreadSends(mocks, rec, entity.SendResult{}, 2)

	require.NoError(t, n.dispatchSync(ctx, entity.NotifyEvent{
		Kind:       entity.NotifyEventStepStarted,
		MaintID:    maintID,
		MaintTitle: "DB upgrade",
	}))

	byChannel := map[string]*entity.MessageRef{}
	for _, sent := range rec.all() {
		byChannel[sent.channel] = sent.replyTo
	}

	require.NotNil(t, byChannel["C123"])
	assert.Equal(t, "1503435956.000247", byChannel["C123"].MessageID)
	require.NotNil(t, byChannel["-100500"])
	assert.Equal(t, "41", byChannel["-100500"].MessageID,
		"each channel must reply to its own root, never another channel's")
}

// A reply that threaded successfully must NOT become the new root.
//
// The transport returns a ref for every delivered message, threaded or not, so
// "there is a ref" cannot be the signal to record one — only RootReplaced can.
// Treating a non-nil ref as the trigger would re-seed the root on every step
// event, breaking the thread into a chain of one-message threads: exactly the
// outcome threading exists to prevent. Any SetRootRef call here fails the test
// as an unexpected gomock call.
func TestDispatchSuccessfulReplyKeepsTheOriginalRoot(t *testing.T) {
	ctx := context.Background()

	n, mocks := initNotifier(t)
	expectNoMentions(mocks)
	maintID := uuid.New()

	tgt := threadedTarget(entity.NotifyTransportSlack, "C123", "1503435956.000247")

	mocks.notifyTarget.EXPECT().
		ListByMaint(gomock.Any(), maintID).
		Return([]*entity.NotifyTarget{tgt}, nil)

	// A threaded reply carries its own new ts — different from the root's, and
	// non-nil. RootReplaced stays false because the root worked.
	rec := &recorder{}
	recordThreadSends(mocks, rec, entity.SendResult{
		Ref: &entity.MessageRef{MessageID: "1503436000.000999"},
	}, 1)

	require.NoError(t, n.dispatchSync(ctx, entity.NotifyEvent{
		Kind:    entity.NotifyEventStepCompleted,
		MaintID: maintID,
	}))

	sent := rec.all()
	require.Len(t, sent, 1)
	require.NotNil(t, sent[0].replyTo, "the event must thread under the existing root")
	assert.Equal(t, "1503435956.000247", sent[0].replyTo.MessageID)
}

// A target with no root sends flat, without an error and without a warning:
// maintenances that started before this feature existed legitimately have none,
// so this is a normal state rather than a degradation.
func TestDispatchWithoutRootSendsFlat(t *testing.T) {
	ctx := context.Background()

	n, mocks := initNotifier(t)
	expectNoMentions(mocks)

	mocks.notifyTarget.EXPECT().
		ListByMaint(gomock.Any(), gomock.Any()).
		Return([]*entity.NotifyTarget{target(entity.NotifyTransportSlack, "C123")}, nil)

	rec := &recorder{}
	recordThreadSends(mocks, rec, entity.SendResult{}, 1)

	require.NoError(t, n.dispatchSync(ctx, entity.NotifyEvent{
		Kind:    entity.NotifyEventStepCompleted,
		MaintID: uuid.New(),
	}))

	sent := rec.all()
	require.Len(t, sent, 1)
	assert.Nil(t, sent[0].replyTo)
}

// The catalog address is editable at runtime while the root is frozen at start.
// Replying into a re-pointed channel would answer an unrelated message there,
// so the mismatch sends flat and is logged rather than hidden.
func TestDispatchRepointedChannelSendsFlat(t *testing.T) {
	ctx := context.Background()

	n, mocks := initNotifier(t)
	expectNoMentions(mocks)

	// Started in -100500, the catalog now points the channel at -100600.
	moved := target(entity.NotifyTransportTelegram, "-100600")
	moved.RootRef = &entity.MessageRef{MessageID: "42"}
	moved.RootChannel = "-100500"

	mocks.notifyTarget.EXPECT().
		ListByMaint(gomock.Any(), gomock.Any()).
		Return([]*entity.NotifyTarget{moved}, nil)

	rec := &recorder{}
	recordThreadSends(mocks, rec, entity.SendResult{}, 1)

	require.NoError(t, n.dispatchSync(ctx, entity.NotifyEvent{
		Kind:    entity.NotifyEventMaintCompleted,
		MaintID: uuid.New(),
	}))

	sent := rec.all()
	require.Len(t, sent, 1)
	assert.Nil(t, sent[0].replyTo, "a root from another chat must never be replayed")
}

// Regression: the guard must not compare the API's canonical chat id against
// the catalog's free-form address.
//
// Channels are addressed by operator-typed text — "@ops_alerts", "#ops" — with
// no normalization or format validation anywhere, while the messenger API
// answers with a canonical id ("-1001234567890", "C0DEADBEEF"). An earlier
// version compared those two and therefore found a "mismatch" on every single
// event of every channel not addressed by its exact canonical id, silently
// disabling threading for a supported configuration while logging it as channel
// drift. Nothing in the transport tests could catch it: the two layers only
// disagree at this seam.
func TestDispatchThreadsWhenCanonicalChatIDDiffersFromAddress(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		transport entity.NotifyTransport
		address   string
		canonical string
	}{
		{
			name:      "telegram @channelname",
			transport: entity.NotifyTransportTelegram,
			address:   "@ops_alerts",
			canonical: "-1001234567890",
		},
		{
			name:      "slack #name",
			transport: entity.NotifyTransportSlack,
			address:   "#ops",
			canonical: "C0DEADBEEF",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			n, mocks := initNotifier(t)
			expectNoMentions(mocks)

			tgt := target(tc.transport, tc.address)
			tgt.RootRef = &entity.MessageRef{MessageID: "42"}
			tgt.RootChannel = tc.address

			mocks.notifyTarget.EXPECT().
				ListByMaint(gomock.Any(), gomock.Any()).
				Return([]*entity.NotifyTarget{tgt}, nil)

			rec := &recorder{}
			recordThreadSends(mocks, rec, entity.SendResult{}, 1)

			require.NoError(t, n.dispatchSync(ctx, entity.NotifyEvent{
				Kind:    entity.NotifyEventStepStarted,
				MaintID: uuid.New(),
			}))

			sent := rec.all()
			require.Len(t, sent, 1)
			require.NotNil(t, sent[0].replyTo,
				"the channel never moved, so the event must still thread")
			assert.Equal(t, "42", sent[0].replyTo.MessageID)
		})
	}
}

// A root recorded before root_channel existed has no stored address to compare,
// so it still threads rather than being discarded.
func TestDispatchEmptyRootChannelStillThreads(t *testing.T) {
	ctx := context.Background()

	n, mocks := initNotifier(t)
	expectNoMentions(mocks)

	tgt := target(entity.NotifyTransportSlack, "#ops")
	tgt.RootRef = &entity.MessageRef{MessageID: "1503435956.000247"}
	tgt.RootChannel = ""

	mocks.notifyTarget.EXPECT().
		ListByMaint(gomock.Any(), gomock.Any()).
		Return([]*entity.NotifyTarget{tgt}, nil)

	rec := &recorder{}
	recordThreadSends(mocks, rec, entity.SendResult{}, 1)

	require.NoError(t, n.dispatchSync(ctx, entity.NotifyEvent{
		Kind:    entity.NotifyEventStepStarted,
		MaintID: uuid.New(),
	}))

	sent := rec.all()
	require.Len(t, sent, 1)
	require.NotNil(t, sent[0].replyTo)
	assert.Equal(t, "1503435956.000247", sent[0].replyTo.MessageID)
}

// maint.reminder fires before the maintenance starts and stays flat by design,
// even if a root somehow exists on the target.
func TestDispatchReminderStaysFlat(t *testing.T) {
	ctx := context.Background()

	n, mocks := initNotifier(t)
	expectNoMentions(mocks)

	mocks.notifyTarget.EXPECT().
		ListByMaint(gomock.Any(), gomock.Any()).
		Return([]*entity.NotifyTarget{
			threadedTarget(entity.NotifyTransportSlack, "C123", "1503435956.000247"),
		}, nil)

	rec := &recorder{}
	recordThreadSends(mocks, rec, entity.SendResult{}, 1)

	require.NoError(t, n.dispatchSync(ctx, entity.NotifyEvent{
		Kind:    entity.NotifyEventMaintReminder,
		MaintID: uuid.New(),
	}))

	sent := rec.all()
	require.Len(t, sent, 1)
	assert.Nil(t, sent[0].replyTo, "the reminder predates the root and must stay flat")
}

// RootReplaced re-seeds the root from the message that was delivered without
// one. Dropping the root instead would send every remaining event of this
// maintenance flat — the scattered-channel problem threading exists to solve —
// so the new root regroups them even though it is titled by a mid-lifecycle
// event rather than the maintenance.
func TestDispatchReplacedRootIsReseeded(t *testing.T) {
	ctx := context.Background()

	n, mocks := initNotifier(t)
	expectNoMentions(mocks)
	maintID := uuid.New()

	tgt := threadedTarget(entity.NotifyTransportSlack, "C123", "1503435956.000247")

	mocks.notifyTarget.EXPECT().
		ListByMaint(gomock.Any(), maintID).
		Return([]*entity.NotifyTarget{tgt}, nil)

	rec := &recorder{}
	recordThreadSends(mocks, rec, entity.SendResult{
		Ref:          &entity.MessageRef{MessageID: "1503436000.000999"},
		RootReplaced: true,
	}, 1)

	// The message delivered without a root becomes the new one.
	mocks.notifyTarget.EXPECT().
		SetRootRef(gomock.Any(), maintID, tgt.ChannelID, "1503436000.000999", "C123").Return(nil)

	require.NoError(t, n.dispatchSync(ctx, entity.NotifyEvent{
		Kind:    entity.NotifyEventStepCompleted,
		MaintID: maintID,
	}))
}

// A root write failure is logged and swallowed. Returning it would make goque
// retry the whole task and re-deliver to every target, and a lost root costs
// only a flat thread.
func TestDispatchRootWriteFailureDoesNotStopTheLoop(t *testing.T) {
	ctx := context.Background()

	n, mocks := initNotifier(t)
	expectNoMentions(mocks)
	maintID := uuid.New()

	first := target(entity.NotifyTransportSlack, "C123")
	second := target(entity.NotifyTransportTelegram, "-100500")

	mocks.notifyTarget.EXPECT().
		ListByMaint(gomock.Any(), maintID).
		Return([]*entity.NotifyTarget{first, second}, nil)

	rec := &recorder{}
	ref := entity.MessageRef{MessageID: "1"}
	recordThreadSends(mocks, rec, entity.SendResult{Ref: &ref}, 2)

	mocks.notifyTarget.EXPECT().
		SetRootRef(gomock.Any(), maintID, first.ChannelID, ref.MessageID, gomock.Any()).
		Return(errors.New("db down"))
	mocks.notifyTarget.EXPECT().
		SetRootRef(gomock.Any(), maintID, second.ChannelID, ref.MessageID, gomock.Any()).
		Return(nil)

	require.NoError(t, n.dispatchSync(ctx, entity.NotifyEvent{
		Kind:    entity.NotifyEventMaintStarted,
		MaintID: maintID,
	}),
		"a root write failure must never surface as a dispatch error")

	require.Len(t, rec.all(), 2, "the second target must still be contacted")
}

// Transports without addressable messages — email, stub — answer every send with
// an empty SendResult. maint.started must then record NO root rather than
// persisting a zero-valued reference, and the dispatch must not fail: on a
// use_stub configuration this is the ONLY shape dispatch ever sees.
//
// Any SetRootRef call here fails the test as an unexpected gomock call.
func TestDispatchRefLessTransportRecordsNoRoot(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		transport entity.NotifyTransport
	}{
		{name: "stub", transport: entity.NotifyTransportStub},
		{name: "email", transport: entity.NotifyTransportEmail},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			n, mocks := initNotifier(t)
			expectNoMentions(mocks)
			maintID := uuid.New()

			mocks.notifyTarget.EXPECT().
				ListByMaint(gomock.Any(), maintID).
				Return([]*entity.NotifyTarget{target(tc.transport, "ops@example.test")}, nil)

			rec := &recorder{}
			recordThreadSends(mocks, rec, entity.SendResult{}, 1)

			require.NoError(t, n.dispatchSync(ctx, entity.NotifyEvent{
				Kind:    entity.NotifyEventMaintStarted,
				MaintID: maintID,
			}))

			// SetRootRef is never EXPECTed: gomock fails the test on an
			// unexpected call, which is the assertion that no root was written.
			require.Len(t, rec.all(), 1, "the message must still be delivered")
		})
	}
}

// A maintenance that started before this feature shipped has no root on any
// target, and every remaining event of its lifecycle must still be delivered —
// flat and without an error.
//
// The pre-feature row is the majority case on the day of the deploy, and it is
// the one shape no other test drives end to end: the migration adds the columns
// as NULL with no backfill, so this is what dispatch actually reads.
func TestDispatchPreFeatureMaintenanceDeliversEverythingFlat(t *testing.T) {
	ctx := context.Background()

	// Every lifecycle event that would normally thread under the root.
	kinds := []entity.NotifyEventKind{
		entity.NotifyEventStepStarted,
		entity.NotifyEventStepCompleted,
		entity.NotifyEventStepCancelled,
		entity.NotifyEventMaintCompleted,
		entity.NotifyEventMaintCancelled,
	}

	n, mocks := initNotifier(t)
	expectNoMentions(mocks)
	maintID := uuid.New()

	// Rebuilt per event the way ListByMaint would return it: no root, because
	// the maintenance started before the columns existed.
	mocks.notifyTarget.EXPECT().
		ListByMaint(gomock.Any(), maintID).
		DoAndReturn(func(context.Context, uuid.UUID) ([]*entity.NotifyTarget, error) {
			return []*entity.NotifyTarget{
				target(entity.NotifyTransportSlack, "C123"),
				target(entity.NotifyTransportTelegram, "-100500"),
			}, nil
		}).
		Times(len(kinds))

	rec := &recorder{}
	recordThreadSends(mocks, rec, entity.SendResult{}, len(kinds)*2)

	for _, kind := range kinds {
		require.NoError(t, n.dispatchSync(ctx, entity.NotifyEvent{
			Kind:    kind,
			MaintID: maintID,
		}), "a maintenance with no root must still deliver %s", kind)
	}

	sent := rec.all()
	require.Len(t, sent, len(kinds)*2, "every event must reach every channel")

	for _, msg := range sent {
		assert.Nil(t, msg.replyTo, "there is no root to reply to")
	}
}

// A delivery failure skips the root bookkeeping entirely — there is no message
// to anchor — and the remaining targets are still contacted.
func TestDispatchSendFailureSkipsRootWrite(t *testing.T) {
	ctx := context.Background()

	n, mocks := initNotifier(t)
	expectNoMentions(mocks)
	maintID := uuid.New()

	failing := target(entity.NotifyTransportSlack, "C123")
	ok := target(entity.NotifyTransportTelegram, "-100500")

	mocks.notifyTarget.EXPECT().
		ListByMaint(gomock.Any(), maintID).
		Return([]*entity.NotifyTarget{failing, ok}, nil)

	ref := entity.MessageRef{MessageID: "1"}
	mocks.sender.EXPECT().
		Send(gomock.Any(), entity.NotifyTransportSlack, gomock.Any(), gomock.Any(), gomock.Any()).
		Return(entity.SendResult{}, errors.New("401 revoked"))
	mocks.sender.EXPECT().
		Send(gomock.Any(), entity.NotifyTransportTelegram, gomock.Any(), gomock.Any(), gomock.Any()).
		Return(entity.SendResult{Ref: &ref}, nil)

	// Only the delivered target gets a root; a SetRootRef for the failed one
	// would be an unexpected call.
	mocks.notifyTarget.EXPECT().SetRootRef(gomock.Any(), maintID, ok.ChannelID, ref.MessageID, gomock.Any()).Return(nil)

	require.NoError(t, n.dispatchSync(ctx, entity.NotifyEvent{
		Kind:    entity.NotifyEventMaintStarted,
		MaintID: maintID,
	}))
}

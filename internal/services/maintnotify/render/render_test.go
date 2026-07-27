package render

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

var (
	testMaintID  = uuid.MustParse("11111111-2222-3333-4444-555555555555")
	testOccurred = time.Date(2026, 7, 27, 10, 30, 0, 0, time.UTC)
	testPlanned  = time.Date(2026, 7, 28, 9, 0, 0, 0, time.UTC)
)

func testEvent(kind entity.NotifyEventKind) entity.NotifyEvent {
	return entity.NotifyEvent{
		Kind:         kind,
		OccurredAt:   testOccurred,
		FrontendURL:  "https://maintmode.test",
		MaintID:      testMaintID,
		MaintTitle:   "DB upgrade",
		PlannedStart: testPlanned,
	}
}

func ptr(s string) *string { return &s }

// TestRenderWithoutOwnerUnchanged pins the exact bodies the templates produced
// before the owner mention was introduced. The literals are transcribed from the
// pre-change templates, not re-derived from the current ones, so a stray blank
// line left by the new conditional block fails here instead of shipping.
func TestRenderWithoutOwnerUnchanged(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	const detailsLine = "Details: https://maintmode.test/maintenance/11111111-2222-3333-4444-555555555555"

	tests := []struct {
		name string
		evt  entity.NotifyEvent
		want string
	}{
		{
			name: "started",
			evt:  testEvent(entity.NotifyEventMaintStarted),
			want: "Maintenance started: DB upgrade\n\n" +
				"Started at 2026-07-27 10:30 UTC.\n" + detailsLine,
		},
		{
			name: "completed",
			evt:  testEvent(entity.NotifyEventMaintCompleted),
			want: "Maintenance completed: DB upgrade\n\n" +
				"Completed at 2026-07-27 10:30 UTC.\n" + detailsLine,
		},
		{
			name: "reminder",
			evt:  testEvent(entity.NotifyEventMaintReminder),
			want: "Upcoming maintenance: DB upgrade\n\n" +
				"Scheduled to start at 2026-07-28 09:00 UTC.\n" + detailsLine,
		},
		{
			name: "cancelled without reason",
			evt:  testEvent(entity.NotifyEventMaintCancelled),
			want: "Maintenance cancelled: DB upgrade\n\n" +
				"Cancelled at 2026-07-27 10:30 UTC.\n" + detailsLine,
		},
		{
			name: "cancelled with reason and comment",
			evt: func() entity.NotifyEvent {
				e := testEvent(entity.NotifyEventMaintCancelled)
				e.CancelReason = entity.MaintenanceCancelReasonIncident
				e.CancelReasonComment = "postponed"

				return e
			}(),
			want: "Maintenance cancelled: DB upgrade\n\n" +
				"Cancelled at 2026-07-27 10:30 UTC.\n" +
				"Reason: incident — postponed\n" + detailsLine,
		},
	}

	svc, err := New()
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			msg, err := svc.Render(ctx, entity.NotifyTransportSlack, tt.evt)
			require.NoError(t, err)
			assert.Equal(t, tt.want, msg.Body)
			assert.Equal(t, entity.TextMessageMIME, msg.MessageMIME)
		})
	}
}

// TestRenderStepUnchanged is the regression guard for step events: they share the
// dispatch and render path but must produce byte-identical bodies.
func TestRenderStepUnchanged(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	const detailsLine = "Details: https://maintmode.test/maintenance/11111111-2222-3333-4444-555555555555"

	tests := []struct {
		name string
		kind entity.NotifyEventKind
		want string
	}{
		{
			name: "started",
			kind: entity.NotifyEventStepStarted,
			want: "Step started — DB upgrade\n\nStep #2: Drain connections\n" +
				"Started at 2026-07-27 10:30 UTC.\n" + detailsLine,
		},
		{
			name: "completed",
			kind: entity.NotifyEventStepCompleted,
			want: "Step completed — DB upgrade\n\nStep #2: Drain connections\n" +
				"Completed at 2026-07-27 10:30 UTC.\n" + detailsLine,
		},
		{
			name: "cancelled",
			kind: entity.NotifyEventStepCancelled,
			want: "Step cancelled — DB upgrade\n\nStep #2: Drain connections\n" +
				"Cancelled at 2026-07-27 10:30 UTC.\n" + detailsLine,
		},
	}

	svc, err := New()
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			evt := testEvent(tt.kind)
			evt.StepOrder = 2
			evt.StepDescription = "Drain connections"
			// Step events carry no owner: nothing resolved it, so it stays nil.
			evt.OwnerMention = nil

			msg, err := svc.Render(ctx, entity.NotifyTransportSlack, evt)
			require.NoError(t, err)
			assert.Equal(t, tt.want, msg.Body)
		})
	}
}

// TestRenderOwnerMentionPerTransport is the "not swapped" test: the Slack body
// must never carry the Telegram handle and vice versa.
func TestRenderOwnerMentionPerTransport(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	mention := &entity.UserMention{
		Name:        "Ruslan Kosykh",
		TelegramTag: ptr("@ruslan_tg"),
		SlackTag:    ptr("ruslan.slack"),
	}

	svc, err := New()
	require.NoError(t, err)

	tests := []struct {
		name      string
		transport entity.NotifyTransport
		want      string
		notWant   string
	}{
		{
			name:      "telegram gets the telegram handle",
			transport: entity.NotifyTransportTelegram,
			want:      "Owner: @ruslan_tg",
			notWant:   "ruslan.slack",
		},
		{
			name:      "slack gets the slack handle",
			transport: entity.NotifyTransportSlack,
			want:      "Owner: ruslan.slack",
			notWant:   "@ruslan_tg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			evt := testEvent(entity.NotifyEventMaintStarted)
			evt.OwnerMention = mention

			msg, err := svc.Render(ctx, tt.transport, evt)
			require.NoError(t, err)
			assert.Contains(t, msg.Body, tt.want)
			assert.NotContains(t, msg.Body, tt.notWant)
			assert.Equal(t, entity.TextMessageMIME, msg.MessageMIME)
		})
	}
}

// TestRenderOwnerMentionFallsBackToName covers every degradation: no handle,
// a handle rejected by the sanitizer, and an owner the resolver could not name.
func TestRenderOwnerMentionFallsBackToName(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc, err := New()
	require.NoError(t, err)

	tests := []struct {
		name    string
		mention *entity.UserMention
		want    string
	}{
		{
			name:    "no handle configured",
			mention: &entity.UserMention{Name: "Ruslan Kosykh"},
			want:    "Owner: Ruslan Kosykh",
		},
		{
			name:    "empty handle",
			mention: &entity.UserMention{Name: "Ruslan Kosykh", SlackTag: ptr("")},
			want:    "Owner: Ruslan Kosykh",
		},
		{
			name: "handle carrying a newline is rejected",
			mention: &entity.UserMention{
				Name:     "Ruslan Kosykh",
				SlackTag: ptr("foo\n\nMaintenance cancelled: everything is fine"),
			},
			want: "Owner: Ruslan Kosykh",
		},
		{
			name: "handle carrying a carriage return is rejected",
			mention: &entity.UserMention{
				Name:     "Ruslan Kosykh",
				SlackTag: ptr("foo\rbar"),
			},
			want: "Owner: Ruslan Kosykh",
		},
		{
			name: "handle carrying U+2028 is rejected",
			mention: &entity.UserMention{
				Name:     "Ruslan Kosykh",
				SlackTag: ptr("foo bar"),
			},
			want: "Owner: Ruslan Kosykh",
		},
		{
			name: "slack subteam mass-mention markup is rejected",
			mention: &entity.UserMention{
				Name:     "Ruslan Kosykh",
				SlackTag: ptr("<!subteam^S01|@team>"),
			},
			want: "Owner: Ruslan Kosykh",
		},
		{
			name: "slack channel mass-mention markup is rejected",
			mention: &entity.UserMention{
				Name:     "Ruslan Kosykh",
				SlackTag: ptr("<!channel>"),
			},
			want: "Owner: Ruslan Kosykh",
		},
		{
			name:    "unresolved owner renders the unknown label",
			mention: &entity.UserMention{Name: entity.UnknownUserName},
			want:    "Owner: " + entity.UnknownUserName,
		},
		{
			name:    "blocked owner renders the blocked label without a handle",
			mention: &entity.UserMention{Name: "Ruslan Kosykh [blocked]"},
			want:    "Owner: Ruslan Kosykh [blocked]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			evt := testEvent(entity.NotifyEventMaintStarted)
			evt.OwnerMention = tt.mention

			msg, err := svc.Render(ctx, entity.NotifyTransportSlack, evt)
			require.NoError(t, err)
			assert.Contains(t, msg.Body, tt.want)
		})
	}
}

// TestRenderRejectedTagNeverReachesBody makes the injection scenario explicit:
// the attacker-controlled continuation must not appear anywhere in the message.
func TestRenderRejectedTagNeverReachesBody(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc, err := New()
	require.NoError(t, err)

	evt := testEvent(entity.NotifyEventMaintStarted)
	evt.OwnerMention = &entity.UserMention{
		Name:     "Ruslan Kosykh",
		SlackTag: ptr("foo\n\nMaintenance cancelled: everything is fine"),
	}

	msg, err := svc.Render(ctx, entity.NotifyTransportSlack, evt)
	require.NoError(t, err)
	assert.NotContains(t, msg.Body, "everything is fine")
	assert.NotContains(t, msg.Body, "foo")
}

// TestRenderOwnerMentionNotDecorated pins that the renderer passes the handle
// through verbatim — no "@" is added to a handle typed without one.
func TestRenderOwnerMentionNotDecorated(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc, err := New()
	require.NoError(t, err)

	evt := testEvent(entity.NotifyEventMaintStarted)
	evt.OwnerMention = &entity.UserMention{Name: "Ruslan Kosykh", SlackTag: ptr("ruslan")}

	msg, err := svc.Render(ctx, entity.NotifyTransportSlack, evt)
	require.NoError(t, err)
	assert.Contains(t, msg.Body, "Owner: ruslan\n")
	assert.NotContains(t, msg.Body, "@ruslan")
}

// TestRenderCancelledWithOwnerAndReason exercises the one template where the
// owner block and the pre-existing CancelReason block coexist.
func TestRenderCancelledWithOwnerAndReason(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc, err := New()
	require.NoError(t, err)

	evt := testEvent(entity.NotifyEventMaintCancelled)
	evt.OwnerMention = &entity.UserMention{Name: "Ruslan Kosykh", SlackTag: ptr("ruslan")}
	evt.CancelReason = entity.MaintenanceCancelReasonIncident
	evt.CancelReasonComment = "postponed"

	msg, err := svc.Render(ctx, entity.NotifyTransportSlack, evt)
	require.NoError(t, err)
	assert.Equal(t,
		"Maintenance cancelled: DB upgrade\n\n"+
			"Owner: ruslan\n\n"+
			"Cancelled at 2026-07-27 10:30 UTC.\n"+
			"Reason: incident — postponed\n"+
			"Details: https://maintmode.test/maintenance/11111111-2222-3333-4444-555555555555",
		msg.Body)
}

package render

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

// TestMentionsLinePerTransport pins that each transport gets its own handle and
// never the other's.
func TestMentionsLinePerTransport(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	mentions := []*entity.UserMention{
		{Name: "Alice", TelegramTag: ptr("@alice_tg"), SlackTag: ptr("alice.slack")},
		{Name: "Bob", TelegramTag: ptr("@bob_tg"), SlackTag: ptr("bob.slack")},
	}

	assert.Equal(t, "@alice_tg, @bob_tg", mentionsLine(ctx, entity.NotifyTransportTelegram, mentions))
	assert.Equal(t, "alice.slack, bob.slack", mentionsLine(ctx, entity.NotifyTransportSlack, mentions))
}

// TestMentionsLineFallsBackToName covers every per-person degradation.
func TestMentionsLineFallsBackToName(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tests := []struct {
		name    string
		mention *entity.UserMention
		want    string
	}{
		{
			name:    "no handle configured falls back to the name",
			mention: &entity.UserMention{Name: "Alice"},
			want:    "Alice",
		},
		{
			name:    "empty handle falls back to the name",
			mention: &entity.UserMention{Name: "Alice", SlackTag: ptr("")},
			want:    "Alice",
		},
		{
			name:    "rejected handle falls back to the name",
			mention: &entity.UserMention{Name: "Alice", SlackTag: ptr("<!channel>")},
			want:    "Alice",
		},
		{
			name:    "the name is sanitized on the fallback path",
			mention: &entity.UserMention{Name: "Иван <!channel>"},
			want:    "Иван &lt;!channel&gt;",
		},
		{
			name:    "an unresolved person renders the unknown label",
			mention: &entity.UserMention{Name: entity.UnknownUserName},
			want:    entity.UnknownUserName,
		},
		{
			name:    "a legitimate ampersand name is escaped, not dropped",
			mention: &entity.UserMention{Name: "Johnson & Johnson Ops"},
			want:    "Johnson & Johnson Ops",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, mentionsLine(ctx, entity.NotifyTransportSlack, []*entity.UserMention{tt.mention}))
		})
	}
}

// TestMentionsLineEmptyInputs pins that nothing to mention renders nothing, so
// the template's {{if .Mentions}} stays false.
func TestMentionsLineEmptyInputs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	assert.Empty(t, mentionsLine(ctx, entity.NotifyTransportSlack, nil))
	assert.Empty(t, mentionsLine(ctx, entity.NotifyTransportSlack, []*entity.UserMention{}))
	// A nil element must not render a stray separator either.
	assert.Empty(t, mentionsLine(ctx, entity.NotifyTransportSlack, []*entity.UserMention{nil}))
}

// TestRenderMentionsInAllFourTemplates pins that the line reaches every
// maintenance template — a mention rendered in three of four would be a silent
// hole in exactly one notification type.
func TestRenderMentionsInAllFourTemplates(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc, err := New()
	require.NoError(t, err)

	kinds := []entity.NotifyEventKind{
		entity.NotifyEventMaintStarted,
		entity.NotifyEventMaintCompleted,
		entity.NotifyEventMaintCancelled,
		entity.NotifyEventMaintReminder,
	}

	for _, kind := range kinds {
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()

			evt := testEvent(kind)
			evt.OwnerMention = &entity.UserMention{Name: "Ruslan Kosykh", SlackTag: ptr("ruslan.slack")}
			evt.Mentions = []*entity.UserMention{
				{Name: "Alice", SlackTag: ptr("alice.slack")},
				{Name: "Bob"},
			}

			msg, err := svc.Render(ctx, entity.NotifyTransportSlack, evt)
			require.NoError(t, err)
			assert.Contains(t, msg.Body, "Owner: ruslan.slack")
			assert.Contains(t, msg.Body, "Mentions: alice.slack, Bob")
		})
	}
}

// TestRenderNoMentionsHeaderWhenEmpty is the shadow-field guard. If templateData
// carried the event's slice instead of the pre-rendered string, {{if .Mentions}}
// would be true for a non-empty slice of fully filtered-out people and emit a
// bare "Mentions:" header with nothing after it.
func TestRenderNoMentionsHeaderWhenEmpty(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc, err := New()
	require.NoError(t, err)

	tests := []struct {
		name     string
		mentions []*entity.UserMention
	}{
		{name: "nil slice", mentions: nil},
		{name: "empty slice", mentions: []*entity.UserMention{}},
		{name: "non-empty slice that filters to empty", mentions: []*entity.UserMention{nil, nil}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			evt := testEvent(entity.NotifyEventMaintStarted)
			evt.Mentions = tt.mentions

			msg, err := svc.Render(ctx, entity.NotifyTransportSlack, evt)
			require.NoError(t, err)
			assert.NotContains(t, msg.Body, "Mentions")
		})
	}
}

// TestRenderWithoutMentionsIsByteIdentical is acceptance criterion 7 for the new
// block: a maintenance with no mentions must produce exactly today's body.
func TestRenderWithoutMentionsIsByteIdentical(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc, err := New()
	require.NoError(t, err)

	evt := testEvent(entity.NotifyEventMaintStarted)
	evt.OwnerMention = &entity.UserMention{Name: "Ruslan Kosykh", SlackTag: ptr("ruslan.slack")}

	msg, err := svc.Render(ctx, entity.NotifyTransportSlack, evt)
	require.NoError(t, err)
	assert.Equal(t,
		"Maintenance started: DB upgrade\n\n"+
			"Owner: ruslan.slack\n\n"+
			"Started at 2026-07-27 10:30 UTC.\n"+
			"Details: https://maintmode.test/maintenance/11111111-2222-3333-4444-555555555555",
		msg.Body)
}

// TestRenderMentionsFullBody pins the exact placement of the new block relative
// to the owner line and the rest of the message.
func TestRenderMentionsFullBody(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc, err := New()
	require.NoError(t, err)

	evt := testEvent(entity.NotifyEventMaintStarted)
	evt.OwnerMention = &entity.UserMention{Name: "Ruslan Kosykh", SlackTag: ptr("ruslan.slack")}
	evt.Mentions = []*entity.UserMention{
		{Name: "Alice", SlackTag: ptr("alice.slack")},
		{Name: "Bob"},
	}

	msg, err := svc.Render(ctx, entity.NotifyTransportSlack, evt)
	require.NoError(t, err)
	assert.Equal(t,
		"Maintenance started: DB upgrade\n\n"+
			"Owner: ruslan.slack\n\n"+
			"Mentions: alice.slack, Bob\n\n"+
			"Started at 2026-07-27 10:30 UTC.\n"+
			"Details: https://maintmode.test/maintenance/11111111-2222-3333-4444-555555555555",
		msg.Body)
}

// TestRenderMentionsWithoutOwner covers the block standing alone: an event with
// mentions but no owner must not leave a dangling blank line where the owner
// block would have been.
func TestRenderMentionsWithoutOwner(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc, err := New()
	require.NoError(t, err)

	evt := testEvent(entity.NotifyEventMaintStarted)
	evt.Mentions = []*entity.UserMention{{Name: "Alice", SlackTag: ptr("alice.slack")}}

	msg, err := svc.Render(ctx, entity.NotifyTransportSlack, evt)
	require.NoError(t, err)
	assert.Equal(t,
		"Maintenance started: DB upgrade\n\n"+
			"Mentions: alice.slack\n\n"+
			"Started at 2026-07-27 10:30 UTC.\n"+
			"Details: https://maintmode.test/maintenance/11111111-2222-3333-4444-555555555555",
		msg.Body)
}

// TestRenderStepTemplatesIgnoreMentions pins that step notifications stay out of
// the feature even if an event somehow carries mentions.
func TestRenderStepTemplatesIgnoreMentions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc, err := New()
	require.NoError(t, err)

	evt := testEvent(entity.NotifyEventStepStarted)
	evt.StepOrder = 2
	evt.StepDescription = "Drain connections"
	evt.Mentions = []*entity.UserMention{{Name: "Alice", SlackTag: ptr("alice.slack")}}

	msg, err := svc.Render(ctx, entity.NotifyTransportSlack, evt)
	require.NoError(t, err)
	assert.NotContains(t, msg.Body, "Mentions")
	assert.NotContains(t, msg.Body, "alice.slack")
}

// TestRenderMentionsCannotForgeALine is the injection assertion for the mentions
// path: a hostile display name must not be able to add a line of its own.
func TestRenderMentionsCannotForgeALine(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc, err := New()
	require.NoError(t, err)

	evt := testEvent(entity.NotifyEventMaintStarted)
	evt.Mentions = []*entity.UserMention{
		{Name: "Alice\n\nMaintenance cancelled: everything is fine"},
		{Name: "<!channel>"},
	}

	msg, err := svc.Render(ctx, entity.NotifyTransportSlack, evt)
	require.NoError(t, err)
	assert.NotContains(t, msg.Body, "<!channel>")
	assert.Contains(t, msg.Body, "&lt;!channel&gt;")
	assert.Contains(t, msg.Body, "Mentions: AliceMaintenance cancelled: everything is fine, &lt;!channel&gt;")
}

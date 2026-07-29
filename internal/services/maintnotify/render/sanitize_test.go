package render

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

// TestSanitizeDisplayName pins the escape-don't-reject contract. A display name
// arrives from OAuth claims without validation, and this feature makes name
// substitution a routine path for up to ten people per notification, so the name
// has to be made harmless while staying readable.
func TestSanitizeDisplayName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "plain name is untouched",
			in:   "Ruslan Kosykh",
			want: "Ruslan Kosykh",
		},
		{
			// The ampersand injects nothing on its own, and no transport here
			// renders HTML entities — Telegram sends with no parse_mode — so
			// escaping it would show the reader a literal "&amp;".
			name: "ampersand reaches the reader unharmed",
			in:   "Johnson & Johnson Ops",
			want: "Johnson & Johnson Ops",
		},
		{
			name: "slack mass mention becomes visible text",
			in:   "Иван <!channel>",
			want: "Иван &lt;!channel&gt;",
		},
		{
			name: "brackets are escaped around an untouched ampersand",
			in:   "<a & b>",
			want: "&lt;a & b&gt;",
		},
		{
			name: "an already-escaped-looking name is left as it is",
			in:   "&lt;",
			want: "&lt;",
		},
		{
			name: "newline is stripped so a forged Owner line is impossible",
			in:   "Ruslan\n\nOwner: attacker",
			want: "RuslanOwner: attacker",
		},
		{
			name: "carriage return is stripped",
			in:   "Ruslan\rKosykh",
			want: "RuslanKosykh",
		},
		{
			name: "unicode line separator is stripped",
			in:   "Ruslan Kosykh",
			want: "RuslanKosykh",
		},
		{
			name: "unicode paragraph separator is stripped",
			in:   "Ruslan Kosykh",
			want: "RuslanKosykh",
		},
		{
			name: "control characters are stripped",
			in:   "Ruslan\x00\x07Kosykh",
			want: "RuslanKosykh",
		},
		{
			name: "the unknown label passes through unchanged",
			in:   entity.UnknownUserName,
			want: entity.UnknownUserName,
		},
		{
			name: "the blocked label passes through unchanged",
			in:   "Ruslan Kosykh [blocked]",
			want: "Ruslan Kosykh [blocked]",
		},
		{
			name: "empty stays empty",
			in:   "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, sanitizeDisplayName(tt.in))
		})
	}
}

// TestSanitizeDisplayNameNeverUnknownUser is the metric-integrity guard.
// entity.UnknownUserName means "auth could not resolve this id" and is the
// trigger for the unresolved counter; substituting it for a valid but hostile
// name would make that metric lie.
func TestSanitizeDisplayNameNeverUnknownUser(t *testing.T) {
	t.Parallel()

	hostile := []string{
		"<!channel>",
		"<!here|here>",
		"<!subteam^S01|@team>",
		"Ruslan\n\nMaintenance cancelled: everything is fine",
		"&&&<<<>>>",
	}

	for _, in := range hostile {
		got := sanitizeDisplayName(in)
		assert.NotEqual(t, entity.UnknownUserName, got, "input %q must not be replaced by the unknown label", in)
		assert.NotContains(t, got, "<")
		assert.NotContains(t, got, ">")
		assert.NotContains(t, got, "\n")
	}
}

// TestOwnerMentionSanitizesName covers all three branches of ownerMention that
// fall back to the display name: unknown transport, no handle, rejected handle.
func TestOwnerMentionSanitizesName(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	const hostile = "Иван <!channel>"
	const wantEscaped = "Иван &lt;!channel&gt;"

	tests := []struct {
		name      string
		transport entity.NotifyTransport
		mention   *entity.UserMention
	}{
		{
			name:      "unknown transport branch",
			transport: entity.NotifyTransport("future"),
			mention:   &entity.UserMention{Name: hostile, SlackTag: ptr("ruslan.slack")},
		},
		{
			name:      "no handle branch",
			transport: entity.NotifyTransportSlack,
			mention:   &entity.UserMention{Name: hostile},
		},
		{
			name:      "rejected handle branch",
			transport: entity.NotifyTransportSlack,
			mention:   &entity.UserMention{Name: hostile, SlackTag: ptr("<!channel>")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ownerMention(ctx, tt.transport, tt.mention)
			assert.Equal(t, wantEscaped, got)
		})
	}
}

// TestRenderOwnerNameCannotForgeALine is the end-to-end injection assertion: a
// display name carrying newlines must not be able to add lines to the body.
func TestRenderOwnerNameCannotForgeALine(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc, err := New()
	require.NoError(t, err)

	evt := testEvent(entity.NotifyEventMaintStarted)
	evt.OwnerMention = &entity.UserMention{
		Name: "Ruslan\n\nMaintenance cancelled: everything is fine",
	}

	msg, err := svc.Render(ctx, entity.NotifyTransportSlack, evt)
	require.NoError(t, err)

	for _, line := range strings.Split(msg.Body, "\n") {
		assert.NotEqual(t, "Maintenance cancelled: everything is fine", strings.TrimSpace(line),
			"a display name must not be able to forge a whole line")
	}
}

// TestRenderLegitimateAmpersandNameSurvives is criterion 4's guard: sanitizing
// must keep the name readable rather than destroying the information the
// fallback exists to deliver.
//
// Both transports are checked, and Telegram is the one that matters most: it is
// sent with no parse_mode, so anything escaped into an HTML entity here would
// reach the reader as those literal characters. The NotContains assertion is the
// regression guard — escaping "&" passes the Contains check on Slack by accident
// and still ships "&amp;" to every Telegram chat.
func TestRenderLegitimateAmpersandNameSurvives(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc, err := New()
	require.NoError(t, err)

	for _, transport := range []entity.NotifyTransport{
		entity.NotifyTransportSlack,
		entity.NotifyTransportTelegram,
	} {
		t.Run(string(transport), func(t *testing.T) {
			t.Parallel()

			evt := testEvent(entity.NotifyEventMaintStarted)
			evt.OwnerMention = &entity.UserMention{Name: "Johnson & Johnson Ops"}

			msg, err := svc.Render(ctx, transport, evt)
			require.NoError(t, err)
			assert.Contains(t, msg.Body, "Owner: Johnson & Johnson Ops")
			assert.NotContains(t, msg.Body, "&amp;",
				"no transport renders HTML entities, so an escaped ampersand reaches the reader literally")
			assert.NotContains(t, msg.Body, entity.UnknownUserName)
		})
	}
}

package entity_test

import (
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

// maxTagBody is the longest handle body the pattern accepts: one mandatory
// leading alphanumeric plus 62 allowlist characters. The stored value is 63
// characters without a leading "@" and 64 with one.
const maxTagBody = 63

func TestCanonicalMessengerTag(t *testing.T) {
	t.Parallel()

	// Cases that behave identically for both transports: the charset rule and the
	// verbatim-storage rule are shared, only the reserved list differs.
	tests := []struct {
		name  string
		input *string
		want  string
		ok    bool
	}{
		// Stored exactly as typed — the two forms are never converted into each other.
		{name: "with at sign kept", input: lo.ToPtr("@ruslan"), want: "@ruslan", ok: true},
		{name: "without at sign kept", input: lo.ToPtr("ruslan"), want: "ruslan", ok: true},
		{name: "only surrounding space trimmed", input: lo.ToPtr("  @ruslan  "), want: "@ruslan", ok: true},

		// Reset to NULL, not an error.
		{name: "nil resets", input: nil, want: "", ok: true},
		{name: "empty resets", input: lo.ToPtr(""), want: "", ok: true},
		{name: "whitespace resets", input: lo.ToPtr("   "), want: "", ok: true},

		// Degenerate values: this whole class passed the earlier prose rule and
		// would have rendered "Owner: @" into the channel.
		{name: "lone at sign", input: lo.ToPtr("@")},
		{name: "lone dot", input: lo.ToPtr(".")},
		{name: "lone dash", input: lo.ToPtr("-")},
		{name: "lone underscore", input: lo.ToPtr("_")},
		{name: "at sign then dot", input: lo.ToPtr("@.")},
		{name: "at sign then underscore", input: lo.ToPtr("@_")},
		{name: "at sign then dash", input: lo.ToPtr("@-")},

		// Length boundary.
		{
			name:  "max length accepted",
			input: lo.ToPtr(strings.Repeat("a", maxTagBody)),
			want:  strings.Repeat("a", maxTagBody),
			ok:    true,
		},
		{
			name:  "max length with at sign accepted",
			input: lo.ToPtr("@" + strings.Repeat("a", maxTagBody)),
			want:  "@" + strings.Repeat("a", maxTagBody),
			ok:    true,
		},
		{name: "over max length rejected", input: lo.ToPtr(strings.Repeat("a", maxTagBody+1))},

		// Non-ASCII and invisible characters. Length is irrelevant here: charset
		// decides first, so a long Cyrillic string is refused for its characters.
		{name: "cyrillic rejected", input: lo.ToPtr("@руслан")},
		{name: "long cyrillic rejected on charset", input: lo.ToPtr(strings.Repeat("я", 65))},
		{name: "zero width rejected", input: lo.ToPtr("@rus\u200blan")},
		{name: "rtl override rejected", input: lo.ToPtr("@rus\u202elan")},

		// Slack markup and control characters — the real mass-ping and
		// message-forgery vectors.
		{name: "slack subteam markup rejected", input: lo.ToPtr("<!subteam^S01|@team>")},
		{name: "slack channel markup rejected", input: lo.ToPtr("<!channel>")},
		{name: "newline rejected", input: lo.ToPtr("foo\nbar")},
		{name: "carriage return rejected", input: lo.ToPtr("foo\rbar")},
		{name: "line separator rejected", input: lo.ToPtr("foo\u2028bar")},

		// The at sign is only ever a prefix.
		{name: "at sign in the middle rejected", input: lo.ToPtr("ab@cd")},
		{name: "double at sign rejected", input: lo.ToPtr("@@ruslan")},
		{name: "inner space rejected", input: lo.ToPtr("rus lan")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotTelegram, okTelegram := entity.CanonicalTelegramTag(tt.input)
			require.Equal(t, tt.ok, okTelegram, "telegram accept")
			require.Equal(t, tt.want, gotTelegram, "telegram value")

			gotSlack, okSlack := entity.CanonicalSlackTag(tt.input)
			require.Equal(t, tt.ok, okSlack, "slack accept")
			require.Equal(t, tt.want, gotSlack, "slack value")
		})
	}
}

// TestCanonicalMessengerTagReserved pins where the line between a broadcast word
// and a username sits.
//
// Both transports refuse here/channel/everyone. Slack expands them literally;
// Telegram does not, but "Owner: @everyone" in a notification is misleading
// regardless of what the transport does with it, and none of the three is a
// plausible personal handle. The neighboring words that *are* real usernames
// stay accepted on both — that is the false-refusal the narrow list avoids.
func TestCanonicalMessengerTagReserved(t *testing.T) {
	t.Parallel()

	reserved := []string{"@HERE", "here", "Channel", "@channel", "everyone", "@EveryOne"}
	usernames := []string{"@admin", "@group", "@all", "@channels", "@here_now"}

	t.Run("both transports reject reserved", func(t *testing.T) {
		t.Parallel()

		for _, input := range reserved {
			got, ok := entity.CanonicalSlackTag(lo.ToPtr(input))
			require.False(t, ok, "slack must reject %q", input)
			require.Empty(t, got)

			got, ok = entity.CanonicalTelegramTag(lo.ToPtr(input))
			require.False(t, ok, "telegram must reject %q", input)
			require.Empty(t, got)
		}
	})

	// The reservation is exact, not a prefix or substring match: @channels and
	// @here_now are ordinary handles that merely start with a reserved word.
	t.Run("both transports accept lookalike usernames", func(t *testing.T) {
		t.Parallel()

		for _, input := range usernames {
			got, ok := entity.CanonicalTelegramTag(lo.ToPtr(input))
			require.True(t, ok, "telegram must accept %q", input)
			require.Equal(t, input, got)

			got, ok = entity.CanonicalSlackTag(lo.ToPtr(input))
			require.True(t, ok, "slack must accept %q", input)
			require.Equal(t, input, got)
		}
	})
}

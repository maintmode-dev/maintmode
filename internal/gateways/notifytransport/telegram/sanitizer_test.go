package telegram

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

// botToken has the real shape — digits, colon, opaque tail — because the rule
// keys off the "bot" prefix rather than off what follows it.
const botToken = "123456:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw"

func TestSanitizeURLMasksBotToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			// The case this type exists for.
			name: "bot token is masked, method stays readable",
			in:   "https://api.telegram.org/bot" + botToken + "/sendMessage",
			want: "https://api.telegram.org/bot[REDACTED]/sendMessage",
		},
		{
			name: "works on a self-hosted API URL",
			in:   "https://tg.internal:8443/bot" + botToken + "/sendMessage",
			want: "https://tg.internal:8443/bot[REDACTED]/sendMessage",
		},
		{
			// Only the first segment is the credential; a "bot"-prefixed
			// segment further down is a route, not a token.
			name: "later bot-prefixed segments are left alone",
			in:   "https://api.telegram.org/bot" + botToken + "/botinfo",
			want: "https://api.telegram.org/bot[REDACTED]/botinfo",
		},
		{
			name: "falls back to the shared policy when there is no token",
			in:   "https://api.telegram.org/health",
			want: "https://api.telegram.org/health",
		},
		{
			name: "unparseable yields the marker",
			in:   "://" + botToken,
			want: "[REDACTED]",
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

			got := sanitizer{}.SanitizeURL(tt.in)

			assert.Equal(t, tt.want, got)
			assert.NotContains(t, got, botToken, "the token must never survive into a log line")
		})
	}
}

// The embedded policy must still apply: this type overrides SanitizeURL only,
// and header redaction is shared so that adding a name in one place covers both.
func TestSanitizerInheritsHeaderPolicy(t *testing.T) {
	t.Parallel()

	got := sanitizer{}.SanitizeHeaders(http.Header{
		"Authorization": []string{"Bearer " + botToken},
		"Content-Type":  []string{"application/json"},
	})

	assert.Equal(t, []string{"[REDACTED]"}, got["Authorization"])
	assert.Equal(t, []string{"application/json"}, got["Content-Type"])
}

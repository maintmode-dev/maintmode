package telegram

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

// New fails fast on an empty bot token: the underlying go-telegram library
// rejects an empty token at construction ("empty token"), so an enabled-but-
// unprovisioned telegram integration surfaces the misconfiguration when the
// transport is built rather than silently at first send.
func TestNew_EmptyToken_FailsFast(t *testing.T) {
	t.Parallel()

	_, err := New(Params{})
	require.Error(t, err, "empty token must fail construction")
}

// A syntactically valid token builds a usable client with the right transport id.
func TestNew_ValidToken_BuildsClient(t *testing.T) {
	t.Parallel()

	c, err := New(Params{BotToken: "123456:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"})
	require.NoError(t, err)
	require.NotNil(t, c)
	require.Equal(t, entity.NotifyTransportTelegram, c.TransportID())
}

// A construction error must never embed the bot token — it flows to xlog.Error at
// the call site. The only error the library returns at build time is "empty token"
// (no secret in it); this locks that property so a future lib bump that starts
// echoing the token in its error is caught here.
func TestNew_Error_NeverContainsToken(t *testing.T) {
	t.Parallel()

	for _, tok := range []string{"", "no-colon-secret", "123:AAA-secret-suffix", ":only-suffix-secret"} {
		if _, err := New(Params{BotToken: tok}); err != nil && tok != "" {
			require.NotContains(t, err.Error(), tok,
				"construction error must not leak the token %q", tok)
		}
	}
}

// Params carries a secret (BotToken). Lock in that it defines no Stringer or JSON
// marshaler so it can never be accidentally logged or serialized in full.
func TestParams_HasNoSecretLeakingSerializer(t *testing.T) {
	t.Parallel()

	var p any = Params{BotToken: "telegram-secret"}
	_, isStringer := p.(fmt.Stringer)
	require.False(t, isStringer, "Params must not implement fmt.Stringer (would risk logging the token)")
	_, isMarshaler := p.(json.Marshaler)
	require.False(t, isMarshaler, "Params must not implement json.Marshaler (would risk serializing the token)")
}

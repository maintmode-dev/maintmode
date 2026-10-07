package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/ruko1202/xhttp/dialguard"
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

// countingServer answers every request with handler and counts the requests
// that reached it.
func countingServer(t *testing.T, handler http.HandlerFunc) (url string, hits *atomic.Int64) {
	t.Helper()

	hits = new(atomic.Int64)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		handler(w, r)
	}))
	t.Cleanup(srv.Close)

	return srv.URL, hits
}

const okSendMessage = `{"ok":true,"result":{"message_id":1,"date":0,"chat":{"id":-100500,"type":"group"}}}`

// api_url is typed by an admin. By default it must not reach an internal
// address: the server below is live and would answer, so the only thing that
// can keep the request from arriving is the dial guard.
func TestNew_RefusesAnInternalAPIURLByDefault(t *testing.T) {
	t.Parallel()

	url, hits := countingServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(okSendMessage))
	})

	c, err := New(Params{BotToken: leakTestToken, APIURL: url})
	require.NoError(t, err)

	_, err = c.Send(context.Background(), "-100500", testMsg, nil)

	require.ErrorContains(t, err, dialguard.ErrBlockedAddress.Error())
	require.NotContains(t, err.Error(), leakTestToken)
	require.Zero(t, hits.Load())
}

// The request path carries the bot token, so a redirect would hand it to a
// host the admin never named.
//
// 302, not 307: the SDK streams its body through a pipe, which net/http cannot
// replay, so a 307 is never followed anyway. A 302 is followed as a bodiless
// GET -- to a URL still carrying the token.
func TestNew_DoesNotFollowRedirects(t *testing.T) {
	t.Parallel()

	target, targetHits := countingServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(okSendMessage))
	})
	origin, _ := countingServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target+r.URL.Path, http.StatusFound)
	})

	c, err := New(Params{BotToken: leakTestToken, APIURL: origin, AllowInternalHosts: true})
	require.NoError(t, err)

	_, err = c.Send(context.Background(), "-100500", testMsg, nil)

	require.Error(t, err)
	require.Zero(t, targetHits.Load(), "the redirect must not be followed")
}

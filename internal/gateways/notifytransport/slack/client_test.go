package slack

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

// New must stay resilient when the bot token is absent (e.g. the integration is
// enabled in the DB but not yet fully provisioned): it returns a usable, non-nil
// client with the right transport id and defers the "no token" failure to send
// time rather than panicking or failing at construction.
func TestNew_EmptyToken_StaysResilient(t *testing.T) {
	t.Parallel()

	c := New(Params{})
	require.NotNil(t, c, "empty token must still yield a non-nil client")
	require.Equal(t, entity.NotifyTransportSlack, c.TransportID())
}

// Params carries a secret (BotToken). Lock in that it defines no Stringer or JSON
// marshaler so it can never be accidentally logged or serialized in full — the
// same at-rest guarantee the type comment asserts, enforced against future edits.
func TestParams_HasNoSecretLeakingSerializer(t *testing.T) {
	t.Parallel()

	var p any = Params{BotToken: "xoxb-secret"}
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

	return srv.URL + "/", hits
}

// api_url is typed by an admin. By default it must not reach an internal
// address: the server below is live and would answer, so the only thing that
// can keep the request from arriving is the dial guard.
func TestNew_RefusesAnInternalAPIURLByDefault(t *testing.T) {
	t.Parallel()

	url, hits := countingServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(okResponse("1.1")))
	})

	_, err := New(Params{BotToken: "xoxb-test", APIURL: url}).Send(context.Background(), "C123", testMsg, nil)

	require.ErrorContains(t, err, dialguard.ErrBlockedAddress.Error())
	require.Zero(t, hits.Load())
}

// A redirect would carry the request, bot token included, to a host the admin
// never named.
func TestNew_DoesNotFollowRedirects(t *testing.T) {
	t.Parallel()

	target, targetHits := countingServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(okResponse("1.1")))
	})
	origin, _ := countingServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target+"chat.postMessage", http.StatusTemporaryRedirect)
	})

	_, err := New(Params{BotToken: "xoxb-test", APIURL: origin, AllowInternalHosts: true}).
		Send(context.Background(), "C123", testMsg, nil)

	require.Error(t, err)
	require.Zero(t, targetHits.Load(), "the redirect must not be followed")
}

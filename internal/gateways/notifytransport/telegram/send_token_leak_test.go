package telegram

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// leakTestToken is a made-up bot token shaped like a real one. It is the needle
// every assertion in this file looks for.
const leakTestToken = "123456:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw"

// deadAPIURL returns the base URL of an httptest server that has already been
// closed, so its port is bound by nothing and the request fails at the transport
// level (connection refused) rather than with an API-level error. That is the
// only failure mode that produces a *url.Error, which is where the token would
// otherwise surface.
func deadAPIURL(t *testing.T) string {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()

	return url
}

// The bot token travels in the URL *path*
// (https://api.telegram.org/bot<TOKEN>/sendMessage), so a transport-level
// failure produces a *url.Error — built inside net/http's Client.do, above our
// own transport — whose Error() string embeds the full URL including the token.
//
// Nothing in this repository scrubs it. The only thing that keeps the token out
// of the error is third-party code: go-telegram/bot v1.22.0 (pinned in
// go.mod:14), raw_request.go:78-81, which does
//
//	var netErr *url.Error
//	if errors.As(errDo, &netErr) {
//	    netErr.URL = strings.Replace(netErr.URL, b.token, "***", -1)
//	}
//
// That error is logged verbatim (internal/gateways/notifytransport/telegram/send.go:38,
// xlog.Error) and persisted by the async sender processor into goque_task.errors,
// so a regression writes the live bot token to logs and to the database.
//
// A RED result here almost certainly means a dependency upgrade dropped or moved
// that scrubbing — not that this package's code broke. The fix in that case is to
// scrub the token in our own Send path (or pin the library back), not to relax
// this test.
func TestSend_TransportError_NeverLeaksBotToken(t *testing.T) {
	t.Parallel()

	c, err := New(Params{
		BotToken: leakTestToken,
		APIURL:   deadAPIURL(t),
	})
	require.NoError(t, err)

	res, sendErr := c.Send(context.Background(), "-100500", testMsg, nil)

	// Positive assertions first, so the leak check below can never pass
	// vacuously on a nil or empty error.
	require.Error(t, sendErr, "an unreachable API must fail the send")
	require.Nil(t, res.Ref, "a failed send has no message ref")
	assert.Contains(t, sendErr.Error(), "sendMessage",
		"the error must still name the failing method for operators")

	assert.NotContains(t, sendErr.Error(), leakTestToken,
		"the bot token must never reach the error text — it is logged and persisted; "+
			"see the comment above: this most likely means go-telegram/bot dropped its "+
			"url.Error scrubbing (raw_request.go:78-81 in v1.22.0)")
}

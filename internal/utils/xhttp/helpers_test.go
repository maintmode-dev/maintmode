package xhttp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ruko1202/xlog"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// secret is the needle. Most assertions in this package are about its absence, so it
// must never appear in an expected value.
const secret = "S3CRETVALUE"

// The token shape is telegram's (digits, colon, base64ish) because that is the one
// caller whose secret sits in the path.
const testToken = "123456:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw"

// observedContext returns a context carrying a logger whose every record is
// captured, plus a function rendering all of them as one string.
//
// The level is Debug on purpose: these assertions are about xlog.Debug output, and
// running them at info would mean asserting that a silent logger stays silent —
// proving nothing while looking green.
func observedContext(t *testing.T) (ctx context.Context, dumpLogs func() string) {
	t.Helper()

	core, logs := observer.New(zapcore.DebugLevel)
	ctx = xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zap.New(core)))

	return ctx, func() string {
		var b strings.Builder
		for _, entry := range logs.All() {
			b.WriteString(entry.Message)
			// Fields carry the payload; encoding them as JSON keeps nested maps
			// (headers) searchable as plain text.
			encoded, err := json.Marshal(entry.ContextMap())
			require.NoError(t, err)
			b.Write(encoded)
			b.WriteString("\n")
		}

		return b.String()
	}
}

// recordedRequest captures what actually crossed the wire, so tests can assert on
// that rather than on what the client believes it sent. Both fields are written by
// the handler goroutine and read after the round trip completes, which the client's
// own happens-before edge makes safe.
type recordedRequest struct {
	header http.Header
	body   []byte
}

// echoServer replies 200 with a fixed JSON body plus a Set-Cookie carrying the
// secret, and records the request it saw.
//
// The Set-Cookie is not incidental: it is the response-side needle for the
// redaction tests, which is why the blocklist is a union of both directions.
func echoServer(t *testing.T) (srvURL string, seen *recordedRequest) {
	t.Helper()

	seen = &recordedRequest{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.header = r.Header.Clone()
		seen.body, _ = io.ReadAll(r.Body)

		w.Header().Set("Set-Cookie", "session="+secret)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)

	return srv.URL, seen
}

// deadServerURL returns an address that is routable but has nothing listening, so a
// request to it fails at dial inside the transport.
func deadServerURL(t *testing.T) string {
	t.Helper()

	dead := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := dead.URL
	dead.Close()

	return url
}

// redirectServer serves a 302 at /start pointing at /end, and 200 "arrived" at
// /end, so redirect options can be judged by what the client actually did rather
// than by whether a function field is non-nil.
func redirectServer(t *testing.T) string {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/end", http.StatusFound)
	})
	mux.HandleFunc("/end", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("arrived"))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return srv.URL
}

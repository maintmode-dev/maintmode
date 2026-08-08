package slack

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	slackgo "github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

// postCall is one recorded chat.postMessage request.
type postCall struct {
	channel  string
	threadTS string
}

// slackStub stands in for the Slack API. The real client is built against it
// through Params.APIURL, so the tests drive the production send path — option
// assembly, response parsing and error classification included — rather than a
// hand-rolled double of it.
type slackStub struct {
	mu       sync.Mutex
	calls    []postCall
	respond  func(call int) (status int, body string)
	headers  map[string]string
	serveURL string
}

func newSlackStub(t *testing.T, respond func(call int) (status int, body string)) *slackStub {
	t.Helper()

	stub := &slackStub{respond: respond}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())

		stub.mu.Lock()
		n := len(stub.calls)
		stub.calls = append(stub.calls, postCall{
			channel:  r.FormValue("channel"),
			threadTS: r.FormValue("thread_ts"),
		})
		headers := stub.headers
		stub.mu.Unlock()

		status, body := stub.respond(n)
		for k, v := range headers {
			w.Header().Set(k, v)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	stub.serveURL = srv.URL + "/"

	return stub
}

func (s *slackStub) recorded() []postCall {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]postCall(nil), s.calls...)
}

func (s *slackStub) client() *Client {
	return New(Params{BotToken: "xoxb-test", APIURL: s.serveURL})
}

// okResponse is the Slack success envelope. The channel is fixed: nothing reads
// it back any more — the delivered chat is logged, not stored — so only ts,
// which a later reply is threaded under, varies per case.
func okResponse(ts string) string {
	return `{"ok":true,"channel":"C123","ts":"` + ts + `"}`
}

var testMsg = entity.NotifyMessage{Subject: "Maintenance started", Body: "DB upgrade"}

// THE regression test for this feature.
//
// PostMessageContext returns the ts of the message it just CREATED, so a
// successful threaded reply comes back with its own ts, always different from
// the thread_ts it was sent under. Any implementation that infers "the root is
// dead" by comparing the returned ref to the requested replyTo would therefore
// clear the root after the first correctly threaded message, and every thread
// would live exactly one reply — while looking perfectly healthy in production
// and passing every other test in this file.
func TestSend_SuccessfulThreadedReplyKeepsTheRoot(t *testing.T) {
	t.Parallel()

	const rootTS = "1503435956.000247"
	const replyTS = "1503435970.000300"

	stub := newSlackStub(t, func(int) (int, string) {
		return http.StatusOK, okResponse(replyTS)
	})

	res, err := stub.client().Send(context.Background(), "C123", testMsg,
		&entity.MessageRef{MessageID: rootTS})
	require.NoError(t, err)

	require.False(t, res.RootReplaced,
		"a successful threaded reply must never report the root as rejected")
	require.NotNil(t, res.Ref)
	assert.Equal(t, replyTS, res.Ref.MessageID,
		"the ref must carry the NEW message ts, not the thread_ts")
	assert.NotEqual(t, rootTS, res.Ref.MessageID)

	calls := stub.recorded()
	require.Len(t, calls, 1, "a successful threaded post must not be retried")
	assert.Equal(t, rootTS, calls[0].threadTS, "thread_ts must carry the root message id")
}

// Without a root the message goes out top-level and carries no thread_ts.
func TestSend_WithoutReplyToSendsFlat(t *testing.T) {
	t.Parallel()

	stub := newSlackStub(t, func(int) (int, string) {
		return http.StatusOK, okResponse("1503435956.000247")
	})

	res, err := stub.client().Send(context.Background(), "C123", testMsg, nil)
	require.NoError(t, err)
	require.False(t, res.RootReplaced)
	require.NotNil(t, res.Ref)

	calls := stub.recorded()
	require.Len(t, calls, 1)
	assert.Empty(t, calls[0].threadTS, "a message with no root must not set thread_ts")
}

// A failed threaded post falls back to exactly ONE flat retry, and reports the
// root as rejected so the caller can drop it. Delivery outranks threading: the
// message still goes out.
func TestSend_ThreadFailureRetriesFlatOnce(t *testing.T) {
	t.Parallel()

	stub := newSlackStub(t, func(call int) (int, string) {
		if call == 0 {
			return http.StatusOK, `{"ok":false,"error":"thread_not_found"}`
		}

		return http.StatusOK, okResponse("1503435999.000111")
	})

	res, err := stub.client().Send(context.Background(), "C123", testMsg,
		&entity.MessageRef{MessageID: "1503435956.000247"})
	require.NoError(t, err, "the message must still be delivered when the thread fails")
	require.True(t, res.RootReplaced)
	require.NotNil(t, res.Ref)
	assert.Equal(t, "1503435999.000111", res.Ref.MessageID)

	calls := stub.recorded()
	require.Len(t, calls, 2, "exactly one flat retry")
	assert.Equal(t, "1503435956.000247", calls[0].threadTS)
	assert.Empty(t, calls[1].threadTS, "the retry must drop thread_ts")
}

// Rate limiting is classified by TYPE (*slack.RateLimitedError via errors.As),
// never by matching the error string. Throttling says nothing about the root:
// retrying flat would discard a healthy thread and double the request rate
// against a workspace that has already hit its limit.
func TestSend_RateLimitedKeepsTheRootAndDoesNotRetry(t *testing.T) {
	t.Parallel()

	stub := newSlackStub(t, func(int) (int, string) {
		return http.StatusTooManyRequests, `{"ok":false,"error":"rate_limited"}`
	})
	// Retry-After is what makes the SDK raise the typed *RateLimitedError rather
	// than a generic StatusCodeError — the type this classification depends on.
	stub.headers = map[string]string{"Retry-After": "30"}

	res, err := stub.client().Send(context.Background(), "C123", testMsg,
		&entity.MessageRef{MessageID: "1503435956.000247"})
	require.Error(t, err, "rate limiting must surface as an error, not a silent flat send")

	var rateLimited *slackgo.RateLimitedError
	require.ErrorAs(t, err, &rateLimited,
		"the typed rate-limit error must survive wrapping so callers can back off")
	assert.False(t, res.RootReplaced, "throttling must never be read as a dead root")
	assert.Nil(t, res.Ref)

	require.Len(t, stub.recorded(), 1, "a rate-limited post must not be retried")
}

// Slack signals throttling in three different shapes and only ONE of them is
// *RateLimitedError: the SDK builds that type solely for a 429 that carries a
// Retry-After header (misc.go checkStatusCode). A 429 without the header
// surfaces as slack.StatusCodeError, and an app-level rate limit arrives as
// HTTP 200 with {"ok":false,"error":"ratelimited"} → slack.SlackErrorResponse.
// Both of the latter are VALUE types, so no pointer-target errors.As matches
// them.
//
// Every shape must be treated as throttling. Misreading one as a dead root
// deletes a healthy thread permanently — nothing ever re-seeds a root — and
// re-posts flat, doubling load on a workspace that is already rate limited.
func TestSend_AllRateLimitShapesKeepTheRoot(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		headers map[string]string
		status  int
		body    string
	}{
		{
			name:    "429 with Retry-After",
			headers: map[string]string{"Retry-After": "30"},
			status:  http.StatusTooManyRequests,
			body:    `{"ok":false,"error":"rate_limited"}`,
		},
		{
			name:   "429 without Retry-After",
			status: http.StatusTooManyRequests,
			body:   `{"ok":false,"error":"rate_limited"}`,
		},
		{
			name:   "application-level ratelimited",
			status: http.StatusOK,
			body:   `{"ok":false,"error":"ratelimited"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			stub := newSlackStub(t, func(int) (int, string) {
				return tc.status, tc.body
			})
			stub.headers = tc.headers

			res, err := stub.client().Send(context.Background(), "C123", testMsg,
				&entity.MessageRef{MessageID: "1503435956.000247"})

			require.Error(t, err)
			assert.False(t, res.RootReplaced,
				"throttling must never be read as a dead root — the root would be deleted forever")
			require.Len(t, stub.recorded(), 1,
				"a throttled post must not be retried against an already-limited workspace")
		})
	}
}

// A transport failure must NOT trigger the fallback: Slack may have published
// the message and lost only the response, so posting again would deliver it
// twice — and re-seed the root onto that duplicate.
//
// The stub accepts the post and then kills the connection without answering,
// which is what a timeout or a dropped connection looks like to the client.
// Classifying by "not throttling" instead of "Slack answered" makes this test
// record two posts.
func TestSend_TransportFailureDoesNotDuplicate(t *testing.T) {
	t.Parallel()

	var (
		mu    sync.Mutex
		posts int
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())

		mu.Lock()
		posts++
		mu.Unlock()

		// The message is accepted; the response never arrives.
		hj, ok := w.(http.Hijacker)
		require.True(t, ok)

		conn, _, err := hj.Hijack()
		require.NoError(t, err)
		require.NoError(t, conn.Close())
	}))
	t.Cleanup(srv.Close)

	client := New(Params{BotToken: "xoxb-test", APIURL: srv.URL + "/"})

	res, err := client.Send(context.Background(), "C123", testMsg,
		&entity.MessageRef{MessageID: "1503435956.000247"})

	require.Error(t, err, "a lost response must surface as a delivery failure")
	assert.False(t, res.RootReplaced, "a transport failure says nothing about the root")

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 1, posts, "the message must be posted once; Slack may already have it")
}

// A flat send that fails is just a delivery failure: no retry, no root verdict.
func TestSend_FlatFailurePropagates(t *testing.T) {
	t.Parallel()

	stub := newSlackStub(t, func(int) (int, string) {
		return http.StatusOK, `{"ok":false,"error":"channel_not_found"}`
	})

	res, err := stub.client().Send(context.Background(), "C123", testMsg, nil)
	require.Error(t, err)
	assert.False(t, res.RootReplaced)
	require.Len(t, stub.recorded(), 1)
}

// RootReplaced is reported for a rejected root and ONLY for one.
//
// The flat retry succeeds, so the user sees a delivered message and the caller
// sees a valid ref — the flag is the only place the lost thread shows up, and
// it is what makes the caller drop a dead root. The negative cases matter as
// much as the positive one: raising it for a healthy send, a throttled one, or
// a plain delivery failure would discard a perfectly good thread.
func TestSend_OnlyRejectedRootIsReported(t *testing.T) {
	t.Parallel()

	const rootTS = "1503435956.000247"

	tests := []struct {
		name         string
		respond      func(call int) (int, string)
		headers      map[string]string
		replyTo      *entity.MessageRef
		wantRejected bool
	}{
		{
			name: "threaded post rejected then retried flat",
			respond: func(call int) (int, string) {
				if call == 0 {
					return http.StatusOK, `{"ok":false,"error":"restricted_action_thread_locked"}`
				}

				return http.StatusOK, okResponse("1503435999.000111")
			},
			replyTo:      &entity.MessageRef{MessageID: rootTS},
			wantRejected: true,
		},
		{
			name: "successful threaded reply",
			respond: func(int) (int, string) {
				return http.StatusOK, okResponse("1503435970.000300")
			},
			replyTo:      &entity.MessageRef{MessageID: rootTS},
			wantRejected: false,
		},
		{
			// Throttling says nothing about the root, so it must not appear
			// under a reason that means "the root is dead".
			name: "rate limited",
			respond: func(int) (int, string) {
				return http.StatusTooManyRequests, `{"ok":false,"error":"rate_limited"}`
			},
			headers:      map[string]string{"Retry-After": "30"},
			replyTo:      &entity.MessageRef{MessageID: rootTS},
			wantRejected: false,
		},
		{
			// No root was requested, so no root can be rejected.
			name: "flat send fails",
			respond: func(int) (int, string) {
				return http.StatusOK, `{"ok":false,"error":"channel_not_found"}`
			},
			wantRejected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			stub := newSlackStub(t, tc.respond)
			stub.headers = tc.headers

			res, _ := stub.client().Send(context.Background(), "C123", testMsg, tc.replyTo)

			assert.Equal(t, tc.wantRejected, res.RootReplaced)
		})
	}
}

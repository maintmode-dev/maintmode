package slack

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

// The bot token travels in the Authorization header and nowhere in the body,
// on the plain post and on the re-post the thread fallback makes alike.
func TestSend_AuthenticatesWithBearerHeaderNotFormBody(t *testing.T) {
	t.Parallel()

	stub := newSlackStub(t, func(call int) (int, string) {
		if call == 0 {
			return http.StatusOK, `{"ok":false,"error":"thread_not_found"}`
		}

		return http.StatusOK, okResponse("2.2")
	})

	_, err := stub.client().Send(context.Background(), "C123", testMsg, &entity.MessageRef{MessageID: "1.1"})
	require.NoError(t, err)

	calls := stub.recorded()
	require.Len(t, calls, 2, "threaded post, then the fallback re-post")
	for i, call := range calls {
		require.Equal(t, "Bearer xoxb-test", call.authorization, "call %d", i)
		require.False(t, call.formToken, "call %d: the token must not be in the form body", i)
		// Stripping the token must not take the message with it.
		require.Equal(t, "C123", call.channel, "call %d", i)
		require.NotEmpty(t, call.text, "call %d", i)
	}
}

// recordingDoer captures what bearerClient hands to the real HTTP client.
type recordingDoer struct{ req *http.Request }

func (d *recordingDoer) Do(req *http.Request) (*http.Response, error) {
	d.req = req

	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
}

func newFormRequest(t *testing.T, form url.Values) *http.Request {
	t.Helper()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		"https://slack.example/api/chat.postMessage", strings.NewReader(form.Encode()))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	return req
}

// A re-sent request (GetBody) must carry the stripped body too, and the
// Content-Length must match what is actually sent.
func TestBearerClient_StripsTokenFromBodyAndResend(t *testing.T) {
	t.Parallel()

	next := &recordingDoer{}
	req := newFormRequest(t, url.Values{"token": {"xoxb-secret"}, "channel": {"C1"}, "text": {"hi"}})

	resp, err := bearerClient{next: next, token: "xoxb-secret"}.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()

	sent := next.req
	require.Equal(t, "Bearer xoxb-secret", sent.Header.Get("Authorization"))

	body, err := io.ReadAll(sent.Body)
	require.NoError(t, err)
	require.NotContains(t, string(body), "xoxb-secret")
	require.Equal(t, int64(len(body)), sent.ContentLength)
	form, err := url.ParseQuery(string(body))
	require.NoError(t, err)
	require.Equal(t, url.Values{"channel": {"C1"}, "text": {"hi"}}, form)

	resent, err := sent.GetBody()
	require.NoError(t, err)
	again, err := io.ReadAll(resent)
	require.NoError(t, err)
	require.Equal(t, body, again)
}

// JSON bodies are not forms: they pass through untouched (slack-go already
// authenticates them with the header).
func TestBearerClient_LeavesNonFormBodiesAlone(t *testing.T) {
	t.Parallel()

	next := &recordingDoer{}
	const payload = `{"channel":"C1","token":"not-a-form-field"}`
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		"https://slack.example/api/chat.postMessage", strings.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := bearerClient{next: next, token: "xoxb-secret"}.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()

	body, err := io.ReadAll(next.req.Body)
	require.NoError(t, err)
	require.JSONEq(t, payload, string(body))
	require.Equal(t, "Bearer xoxb-secret", next.req.Header.Get("Authorization"))
}

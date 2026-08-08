package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

// replyParameters is the decoded reply_parameters form field. The SDK posts
// sendMessage as multipart/form-data and puts this nested object in as a JSON
// string, so it is parsed in two steps.
type replyParameters struct {
	MessageID                int  `json:"message_id"`
	AllowSendingWithoutReply bool `json:"allow_sending_without_reply"`
}

// sendRequest is the decoded sendMessage body, so assertions read the wire
// format the bot actually produced rather than a re-derived expectation.
type sendRequest struct {
	ChatID          string
	Text            string
	ReplyParameters *replyParameters
}

// tgStub stands in for the Telegram Bot API. The real client is built against
// it through Params.APIURL, so the tests exercise the production send path.
type tgStub struct {
	mu       sync.Mutex
	requests []sendRequest
	respond  func() string
	serveURL string
}

func newTGStub(t *testing.T, respond func() string) *tgStub {
	t.Helper()

	stub := &tgStub{respond: respond}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseMultipartForm(1<<20),
			"the SDK posts sendMessage as multipart/form-data")

		req := sendRequest{
			ChatID: r.FormValue("chat_id"),
			Text:   r.FormValue("text"),
		}
		if raw := r.FormValue("reply_parameters"); raw != "" {
			var params replyParameters
			require.NoError(t, json.Unmarshal([]byte(raw), &params))
			req.ReplyParameters = &params
		}

		stub.mu.Lock()
		stub.requests = append(stub.requests, req)
		stub.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(stub.respond()))
	}))
	t.Cleanup(srv.Close)

	stub.serveURL = srv.URL

	return stub
}

func (s *tgStub) recorded() []sendRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]sendRequest(nil), s.requests...)
}

func (s *tgStub) client(t *testing.T) *Client {
	t.Helper()

	c, err := New(Params{
		BotToken: "123456:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		APIURL:   s.serveURL,
	})
	require.NoError(t, err)

	return c
}

// The ids every stubbed delivery comes back with.
const (
	sentMessageID = 42
	sentChatID    = -100500
)

// sentOK is a delivered message that Telegram accepted as a reply: it echoes
// reply_to_message, which is how the client detects the reply was honored.
func sentOK(threaded bool) string {
	chat := strconv.FormatInt(sentChatID, 10)

	reply := ""
	if threaded {
		reply = `,"reply_to_message":{"message_id":41,"chat":{"id":` + chat + `}}`
	}

	return `{"ok":true,"result":{"message_id":` + strconv.Itoa(sentMessageID) +
		`,"chat":{"id":` + chat + `},"text":"x"` + reply + `}}`
}

var testMsg = entity.NotifyMessage{Subject: "Maintenance started", Body: "DB upgrade"}

// A root produces reply_parameters with AllowSendingWithoutReply, which is what
// makes a deleted root degrade to a flat message server-side instead of failing
// the send.
func TestSend_WithReplyToSetsReplyParameters(t *testing.T) {
	t.Parallel()

	stub := newTGStub(t, func() string { return sentOK(true) })

	res, err := stub.client(t).Send(context.Background(), "-100500", testMsg,
		&entity.MessageRef{MessageID: "41"})
	require.NoError(t, err)
	require.False(t, res.RootReplaced)

	reqs := stub.recorded()
	require.Len(t, reqs, 1)
	require.NotNil(t, reqs[0].ReplyParameters)
	assert.Equal(t, 41, reqs[0].ReplyParameters.MessageID)
	assert.True(t, reqs[0].ReplyParameters.AllowSendingWithoutReply,
		"a deleted root must degrade to a flat message instead of failing the send")

	require.NotNil(t, res.Ref)
	assert.Equal(t, "42", res.Ref.MessageID)
}

// Without a root the request carries no reply_parameters at all.
func TestSend_WithoutReplyToOmitsReplyParameters(t *testing.T) {
	t.Parallel()

	stub := newTGStub(t, func() string { return sentOK(false) })

	res, err := stub.client(t).Send(context.Background(), "-100500", testMsg, nil)
	require.NoError(t, err)
	require.False(t, res.RootReplaced)

	reqs := stub.recorded()
	require.Len(t, reqs, 1)
	assert.Nil(t, reqs[0].ReplyParameters, "no root must mean no reply_parameters in the request")
}

// MessageRef.MessageID is a string across the domain because a Slack ts is not
// numeric, so a Slack-shaped id can reach the Telegram client through corrupt
// data. It must send flat rather than panic or drop the message: delivery
// outranks threading.
func TestSend_UnparsableRootIDSendsFlat(t *testing.T) {
	t.Parallel()

	stub := newTGStub(t, func() string { return sentOK(false) })

	res, err := stub.client(t).Send(context.Background(), "-100500", testMsg,
		&entity.MessageRef{MessageID: "1503435956.000247"})
	require.NoError(t, err, "a garbage root id must not fail the delivery")
	require.False(t, res.RootReplaced)
	require.NotNil(t, res.Ref, "the message was delivered, so it has a ref")

	reqs := stub.recorded()
	require.Len(t, reqs, 1)
	assert.Nil(t, reqs[0].ReplyParameters, "an unparsable root must be dropped, not sent")
}

// AllowSendingWithoutReply makes the server deliver a plain message and omit
// reply_to_message when the root is gone. The client detects that, but keeps
// the root: a dropped reply is not proof the root is dead forever, and clearing
// it would throw away a thread that may still work on the next event.
func TestSend_DegradedReplyKeepsTheRoot(t *testing.T) {
	t.Parallel()

	stub := newTGStub(t, func() string { return sentOK(false) })

	res, err := stub.client(t).Send(context.Background(), "-100500", testMsg,
		&entity.MessageRef{MessageID: "41"})
	require.NoError(t, err)
	assert.False(t, res.RootReplaced,
		"telegram must never clear the root on a dropped reply")
	require.NotNil(t, res.Ref)

	reqs := stub.recorded()
	require.Len(t, reqs, 1)
	require.NotNil(t, reqs[0].ReplyParameters, "the reply was requested; the server ignored it")
}

// Both degradation paths in this client cost the thread, never the message: it
// is delivered either way, RootReplaced stays false so the root survives for the
// next event, and an unparsable id is dropped rather than sent on.
func TestSend_DegradationsKeepTheDelivery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		rootID    string
		threaded  bool
		wantReply bool
	}{
		{
			// Telegram accepted the message but omitted reply_to_message:
			// AllowSendingWithoutReply kicked in, usually a deleted root.
			name:      "server dropped the reply",
			rootID:    "41",
			threaded:  false,
			wantReply: true,
		},
		{
			// A Slack-shaped ts reached the Telegram client through corrupt
			// data; it never leaves the process.
			name:     "unparsable root id",
			rootID:   "1503435956.000247",
			threaded: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			stub := newTGStub(t, func() string { return sentOK(tc.threaded) })

			res, err := stub.client(t).Send(context.Background(), "-100500", testMsg,
				&entity.MessageRef{MessageID: tc.rootID})
			require.NoError(t, err, "a degraded thread must never cost the delivery")
			require.NotNil(t, res.Ref, "the message was delivered, so it has a ref")
			assert.False(t, res.RootReplaced,
				"telegram never reports the root as rejected — the next event may still thread")

			reqs := stub.recorded()
			require.Len(t, reqs, 1)
			if tc.wantReply {
				require.NotNil(t, reqs[0].ReplyParameters, "the reply was requested; the server ignored it")
			} else {
				require.Nil(t, reqs[0].ReplyParameters, "an unparsable root must be dropped, not sent")
			}
		})
	}
}

// A send failure surfaces as an error with no ref and no root verdict.
func TestSend_ErrorPropagates(t *testing.T) {
	t.Parallel()

	stub := newTGStub(t, func() string {
		return `{"ok":false,"error_code":400,"description":"chat not found"}`
	})

	res, err := stub.client(t).Send(context.Background(), "-100500", testMsg, nil)
	require.Error(t, err)
	assert.Nil(t, res.Ref)
	assert.False(t, res.RootReplaced)
}

package slack

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	slackgo "github.com/slack-go/slack"

	"github.com/ruko1202/maintmode/internal/entity"
)

// Send delivers msg to target, threading it under replyTo when one is given.
//
// Slack has no equivalent of Telegram's AllowSendingWithoutReply, so the
// fallback is manual: when a threaded post fails, post the SAME message again
// WITHOUT thread_ts and report RootReplaced, which tells the caller the
// delivered message is the one to thread under from now on.
//
// This is not a retry of a failed request in the usual sense — the second call
// is a different request, with the one option that caused the failure removed.
// The errors that reach it (locked thread, non-threadable channel, deleted
// root) are threading failures, so a post without thread_ts does not reproduce
// them.
//
// A failed post is classified in that order: throttling is a plain failure,
// then an explicit refusal from Slack is the only thing that triggers the
// fallback, and everything else is a plain failure too. A transport failure
// must never fall back, because Slack may have already published the message
// and only the response was lost; posting again would deliver it twice and,
// worse, re-seed the root onto the duplicate.
func (c *Client) Send(
	ctx context.Context,
	target string,
	msg entity.NotifyMessage,
	replyTo *entity.MessageRef,
) (entity.SendResult, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "slack.Send")
	defer span.End()

	opts := []slackgo.MsgOption{
		slackgo.MsgOptionText(formatBody(msg), false),
		slackgo.MsgOptionDisableLinkUnfurl(),
	}
	if replyTo != nil {
		opts = append(opts, slackgo.MsgOptionTS(replyTo.MessageID))
	}

	_, ts, err := c.api.PostMessageContext(ctx, target, opts...)
	if err != nil {
		// Throttling is answered before anything else: the second post would fail
		// the same way, doubling the request rate against a workspace already at
		// its limit and discarding a healthy thread with it.
		if isRateLimited(err) {
			xlog.Error(ctx, "slack postMessage rate limited, keeping thread root",
				xfield.Error(err))

			return entity.SendResult{}, fmt.Errorf("slack postMessage: %w", err)
		}

		// Slack answered, and throttling is already ruled out, so the answer was a
		// refusal of the root: re-post without it. Which refusal it is does not
		// matter — Slack does not document the code for a deleted root
		// (thread_not_found and invalid_thread_ts appear in the wild but not in
		// the official chat.postMessage error list), so this never branches on the
		// error string. A false positive costs one flat message; a false negative
		// would silently drop a notification.
		var refused slackgo.SlackErrorResponse
		if replyTo != nil && errors.As(err, &refused) {
			xlog.Warn(ctx, "slack refused the thread root, posting without it",
				xfield.String("root_message_id", replyTo.MessageID),
				xfield.Error(err))

			return c.postWithoutRoot(ctx, target, msg)
		}

		// Anything left is a transport failure, which must NOT fall back: Slack may
		// have published the message and lost only the response, so posting again
		// would deliver it twice and re-seed the root onto the duplicate.
		xlog.Error(ctx, "slack postMessage", xfield.Error(err))

		return entity.SendResult{}, fmt.Errorf("slack postMessage: %w", err)
	}

	// ts is the ts of the message just created — for a threaded reply it is its
	// OWN ts, never the root's. Do not compare it to replyTo.
	return entity.SendResult{Ref: newRef(ts)}, nil
}

// postWithoutRoot re-posts msg with no thread root and reports the delivered
// message as the replacement root. See Send for why this is not a retry.
//
// The recursion is one level deep by construction: the nested call passes a nil
// replyTo, so the fallback branch in Send is not taken and it returns either a
// delivered message or an error.
func (c *Client) postWithoutRoot(
	ctx context.Context,
	target string,
	msg entity.NotifyMessage,
) (entity.SendResult, error) {
	res, err := c.Send(ctx, target, msg, nil)
	if err != nil {
		return entity.SendResult{}, err
	}

	// RootReplaced is stated as a fact by the transport, which is the only layer
	// that knows the threaded attempt failed. It is never inferred from ids, and
	// the nested call cannot know it — hence setting it here.
	res.RootReplaced = true

	return res, nil
}

// isRateLimited reports whether err is Slack throttling, in any of the three
// shapes the SDK produces. Classification is by TYPE, never by matching an
// error string.
//
// One type is not enough: the SDK builds *RateLimitedError only for a 429 that
// carries a Retry-After header (misc.go checkStatusCode). A 429 without the
// header becomes slack.StatusCodeError, and an application-level limit arrives
// as HTTP 200 with {"ok":false,"error":"ratelimited"} → slack.SlackErrorResponse.
// The latter two are VALUE types, so they need value targets — a pointer target
// silently never matches, which is exactly how a healthy thread would get
// deleted on a transient 429.
func isRateLimited(err error) bool {
	var rateLimited *slackgo.RateLimitedError
	if errors.As(err, &rateLimited) {
		return true
	}

	var statusErr slackgo.StatusCodeError
	if errors.As(err, &statusErr) && statusErr.Code == http.StatusTooManyRequests {
		return true
	}

	// Slack spells the application-level code without an underscore
	// ("ratelimited"); the transport-level one is "rate_limited". Matched on the
	// typed Err field of a typed error, not on a formatted message.
	var respErr slackgo.SlackErrorResponse
	if errors.As(err, &respErr) {
		return respErr.Err == "ratelimited" || respErr.Err == "rate_limited"
	}

	return false
}

// newRef carries the ts of the message just created — the value a later reply
// is threaded under.
func newRef(timestamp string) *entity.MessageRef {
	if timestamp == "" {
		return nil
	}

	return &entity.MessageRef{MessageID: timestamp}
}

func formatBody(msg entity.NotifyMessage) string {
	if msg.Subject == "" {
		return msg.Body
	}

	return fmt.Sprintf("*%s*"+
		"\n%s",
		msg.Subject,
		msg.Body,
	)
}

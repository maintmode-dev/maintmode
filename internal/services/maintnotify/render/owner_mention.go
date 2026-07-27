package render

import (
	"context"
	"strings"
	"unicode"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/metrics"
)

// slackMarkupMeta are the characters Slack uses to build markup, most
// dangerously the mass-mention forms <!channel>, <!here|here> and
// <!subteam^S012345|@team-name>. A handle containing any of them is refused: a
// reserved-word check would pass <!subteam^...> unharmed, which is exactly the
// class this barrier exists for.
const slackMarkupMeta = "<>!|^&"

// lineSeparator and paragraphSeparator are Unicode's own line breaks. Neither
// is unicode.IsControl — that category stops at U+009F — yet both break a line
// in most renderers, so they are refused explicitly.
const (
	lineSeparator      = ' '
	paragraphSeparator = ' '
)

// ownerMention picks the handle matching the delivery transport and degrades to
// the display name whenever a usable handle is not available.
//
// Nothing is decorated: the handle is rendered exactly as the user typed it,
// with no "@" prefix added. The empty string is returned for events with no
// owner (step events), and the templates skip the whole block for it.
func ownerMention(ctx context.Context, transport entity.NotifyTransport, mention *entity.UserMention) string {
	if mention == nil {
		return ""
	}

	var tag *string

	switch transport {
	case entity.NotifyTransportTelegram:
		tag = mention.TelegramTag
	case entity.NotifyTransportSlack:
		tag = mention.SlackTag
	default:
		// Only slack and telegram are reachable here: stub fails
		// NotifyTransport.IsValid and email is a system delivery transport, not
		// a subscribable notify channel. This branch guards future transports
		// rather than handling today's.
		return mention.Name
	}

	if tag == nil || *tag == "" {
		// No handle configured. For an owner the resolver could not name at all
		// this is the "unresolved" degradation; otherwise it is simply the
		// switched-off state of the feature, which must NOT be counted — it
		// holds for most users and would keep a rate>0 alert permanently red.
		if mention.Name == entity.UnknownUserName {
			metrics.MaintNotifyOwnerMentionDegraded(ctx, metrics.OwnerMentionUnresolved)
		}

		return mention.Name
	}

	if !isSafeTag(*tag) {
		// Second barrier, for values that reached the DB around input
		// validation (direct SQL, a future migration). A newline inside the
		// handle would inject arbitrary lines into a plain-text notification.
		xlog.Warn(ctx, "owner messenger tag rejected by sanitizer, falling back to name",
			xfield.String("transport", string(transport)))
		metrics.MaintNotifyOwnerMentionDegraded(ctx, metrics.OwnerMentionRejected)

		return mention.Name
	}

	return *tag
}

// isSafeTag rejects two classes only — control characters (line breaks above
// all) and Slack markup metacharacters. It is deliberately not a full charset
// allowlist: the allowlist lives on the input path, and duplicating it here
// would fork two definitions of a valid handle.
func isSafeTag(tag string) bool {
	if strings.ContainsAny(tag, slackMarkupMeta) {
		return false
	}

	for _, r := range tag {
		if unicode.IsControl(r) || r == '\n' || r == '\r' ||
			r == lineSeparator || r == paragraphSeparator {
			return false
		}
	}

	return true
}

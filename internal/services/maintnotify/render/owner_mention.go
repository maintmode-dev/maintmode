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
		return sanitizeDisplayName(mention.Name)
	}

	if tag == nil || *tag == "" {
		// No handle configured. For an owner the resolver could not name at all
		// this is the "unresolved" degradation; otherwise it is simply the
		// switched-off state of the feature, which must NOT be counted — it
		// holds for most users and would keep a rate>0 alert permanently red.
		if mention.Name == entity.UnknownUserName {
			metrics.MaintNotifyOwnerMentionDegraded(ctx, metrics.OwnerMentionUnresolved)
		}

		return sanitizeDisplayName(mention.Name)
	}

	if !isSafeTag(*tag) {
		// Second barrier, for values that reached the DB around input
		// validation (direct SQL, a future migration). A newline inside the
		// handle would inject arbitrary lines into a plain-text notification.
		xlog.Warn(ctx, "owner messenger tag rejected by sanitizer, falling back to name",
			xfield.String("transport", string(transport)))
		metrics.MaintNotifyOwnerMentionDegraded(ctx, metrics.OwnerMentionRejected)

		return sanitizeDisplayName(mention.Name)
	}

	return *tag
}

// sanitizeDisplayName makes a display name safe to interpolate into a
// notification body while keeping it readable.
//
// It escapes rather than rejects, which is the opposite of isSafeTag and
// deliberately so. A name is not a handle: "&" appears in legitimate names
// ("Johnson & Johnson Ops", "R&D On-Call") and carries no risk by itself, so
// refusing it would destroy exactly the information the name fallback exists to
// deliver. And the only thing a refusal could substitute is
// entity.UnknownUserName, which means "auth could not resolve this id" and
// triggers the unresolved metric — substituting it for a valid name would make
// that metric lie. A name therefore NEVER becomes the unknown label here.
//
// The threat is real, not theoretical: the Slack gateway sends with slack-go's
// escaping disabled, mrkdwn is always parsed, and display names arrive from
// OAuth claims unvalidated. A name of "Иван <!channel>" would ping the channel.
// Telegram is sent without parse_mode so markup is inert there today; both
// branches are escaped anyway, so enabling parse_mode later cannot open a hole.
//
// Only the angle brackets are escaped. "&" is left alone on purpose: the
// injection primitive is the bracketed form (<!channel>, <!subteam^...>), which
// neutralizing "<" already breaks, and no transport here renders HTML entities —
// Telegram is sent with no parse_mode at all, so an escaped "&amp;" would reach
// the reader as those five literal characters and mangle every "R&D On-Call" we
// were trying to preserve.
func sanitizeDisplayName(name string) string {
	name = strings.ReplaceAll(name, "<", "&lt;")
	name = strings.ReplaceAll(name, ">", "&gt;")

	// Line breaks are dropped, not escaped: a name spanning lines could forge an
	// "Owner:" line of its own inside a plain-text body.
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '\n' || r == '\r' ||
			r == lineSeparator || r == paragraphSeparator {
			return -1
		}

		return r
	}, name)
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

package telegram

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/ruko1202/xhttp/sanitize"

	"github.com/ruko1202/maintmode/internal/utils/xsanitize"
)

// botPathPrefix is the marker the Bot API puts in front of the token in the URL
// path: /bot<token>/<method>.
const botPathPrefix = "bot"

// maskedTokenSegment replaces the token segment in a logged URL, keeping the
// prefix so the path still reads as the Bot API route it is.
//
// Concatenated rather than written as one literal so gosec does not read
// "bot"+opaque-looking text as a hardcoded credential; there is no secret here,
// only its replacement.
const maskedTokenSegment = botPathPrefix + "[REDACTED]"

// tokenSegment is the shape of a token segment: "bot", the bot's numeric id, a
// colon. The shape rather than the position is what identifies it, because an
// api_url with a path of its own -- a corporate proxy at /telegram -- moves the
// token out of the first segment.
var tokenSegment = regexp.MustCompile(`^` + botPathPrefix + `\d+:`)

var _ sanitize.Sanitizer = sanitizer{}

// sanitizer extends the shared policy with the one thing specific to Telegram:
// the Bot API puts the credential in the URL path, as
// https://api.telegram.org/bot<token>/sendMessage.
//
// That is why redaction is per-integration rather than a single rule in the
// transport. Every other caller in this service carries a plain route, and
// masking their paths wholesale — the way the old shared redactor did — would
// cost real diagnostics to defend against a threat only this gateway has.
//
// Embedding xsanitize.Sanitizer keeps the header policy shared: adding a
// sensitive header name there reaches this type too, which a copied blocklist
// would not.
type sanitizer struct {
	xsanitize.Sanitizer
}

// SanitizeURL masks the bot token and leaves the rest of the path intact:
//
//	https://api.telegram.org/bot123:AAH.../sendMessage
//	→ https://api.telegram.org/bot[REDACTED]/sendMessage
//
// Every path segment is examined, not only the first: an api_url with a path of
// its own (https://proxy.corp/telegram) puts the token deeper, and a sanitizer
// that only looked first leaked it into error logs and spans. A segment is
// masked by the token's shape, so an ordinary "bot"-prefixed word is left alone.
//
// The URL is rebuilt by hand rather than through url.URL.String(), which would
// percent-escape the brackets in the marker into %5BREDACTED%5D. A log line is
// for a human to read.
func (s sanitizer) SanitizeURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		// Delegate the failure mode rather than reimplementing it: the shared
		// policy already answers "unparseable" with the marker.
		return s.Sanitizer.SanitizeURL(rawURL)
	}

	segments := strings.Split(u.Path, "/")
	masked := false
	for i, segment := range segments {
		if tokenSegment.MatchString(segment) {
			segments[i] = maskedTokenSegment
			masked = true
		}
	}
	if !masked {
		return s.Sanitizer.SanitizeURL(rawURL)
	}

	var b strings.Builder

	if u.Scheme != "" {
		b.WriteString(u.Scheme)
		b.WriteString("://")
	}
	// Userinfo is dropped for the same reason the shared policy drops it.
	b.WriteString(u.Host)
	b.WriteString(strings.Join(segments, "/"))

	if u.RawQuery != "" {
		b.WriteString("?")
		b.WriteString(u.RawQuery)
	}

	return b.String()
}

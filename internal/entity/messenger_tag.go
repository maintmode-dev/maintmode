package entity

import (
	"regexp"
	"slices"
	"strings"
)

// messengerTagPattern is the authoritative definition of an acceptable messenger
// handle. The rule is the expression itself, not a prose paraphrase of it: an
// earlier prose formulation ("allowlist plus an optional leading @") admitted a
// whole class of junk — "@", ".", "-", "_", "@.", "@_" — because a lone "@"
// satisfies "length 1 and a leading @ is present", and "Owner: @" went out to the
// channel. Requiring at least one alphanumeric after the optional "@" closes that
// by construction.
//
// The character class is ASCII-only on purpose. The same rule that rejects
// homoglyphs also rejects zero-width characters (U+200B is not unicode.IsSpace,
// so TrimSpace leaves it), RTL overrides, newlines, and every Slack markup
// metacharacter (<, >, !, |, ^, &) — which is what actually blocks mass pings,
// since Slack's real broadcast form is <!channel>, not @channel. Legitimate
// handles fit: Telegram usernames are [A-Za-z0-9_], Slack handles are lowercase
// with . - _.
//
// The {0,62} bound plus the mandatory leading alphanumeric caps the handle at 63
// characters after the optional "@", i.e. 1-64 characters overall.
var messengerTagPattern = regexp.MustCompile(`^@?[A-Za-z0-9][A-Za-z0-9._-]{0,62}$`)

// reservedTags name a whole channel or workspace rather than a person. They are
// refused on both transports.
//
// In Slack these are the literal broadcast words. In Telegram they are not —
// nothing there expands @channel — but neither is any of them a plausible
// personal handle, and a notification reading "Owner: @everyone" misleads
// whatever the transport does with it. The narrower words that *are* real
// usernames — @admin, @group, @all — stay allowed, because refusing those would
// be a false refusal.
var reservedTags = []string{"here", "channel", "everyone"}

// CanonicalTelegramTag reports whether tag is an acceptable Telegram handle and
// returns the value to persist. See canonicalMessengerTag for the shared contract.
func CanonicalTelegramTag(tag *string) (string, bool) {
	return canonicalMessengerTag(tag)
}

// CanonicalSlackTag reports whether tag is an acceptable Slack handle and returns
// the value to persist. See canonicalMessengerTag for the shared contract.
//
// It stays a separate entry point from the Telegram one even though the rules
// now coincide: the two handles are distinct fields with distinct call sites,
// and the transports' rules have already diverged once.
func CanonicalSlackTag(tag *string) (string, bool) {
	return canonicalMessengerTag(tag)
}

// canonicalMessengerTag validates a messenger handle and returns the value to
// persist.
//
// Storage is verbatim: "@ruslan" stays "@ruslan" and "ruslan" stays "ruslan" —
// the two are never converted into one another, because the stored value is what
// gets pasted into the outgoing message. The only edit to the input is
// strings.TrimSpace: surrounding spaces are invisible to the user, break the
// mention in the messenger, and blur "empty" against "whitespace".
//
// Returns:
//   - ("", true)  for nil / empty / whitespace → reset to NULL (not an error);
//   - (tag, true) for an accepted handle       → store the trimmed value as typed;
//   - ("", false) for anything else            → caller maps to apperr.ErrInvalidMessengerTag.
//
// Charset is checked before length: the regexp bounds both, so 65 Cyrillic
// letters are refused as invalid characters rather than as "too long", which is
// the honest reason.
func canonicalMessengerTag(tag *string) (string, bool) {
	if tag == nil {
		return "", true
	}

	trimmed := strings.TrimSpace(*tag)
	if trimmed == "" {
		return "", true
	}

	if !messengerTagPattern.MatchString(trimmed) {
		return "", false
	}

	// Compare with the leading "@" stripped and case folded, but store the value
	// untouched — "@Channel", "channel" and "@channel" are the same reservation.
	compared := strings.ToLower(strings.TrimPrefix(trimmed, "@"))
	if slices.Contains(reservedTags, compared) {
		return "", false
	}

	return trimmed, true
}

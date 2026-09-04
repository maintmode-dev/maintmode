// Package xemail normalizes email addresses so that every consumer of one
// agrees on what "the same address" means.
package xemail

import (
	"strings"
	"unicode"
)

// Normalize trims an address and folds its case.
//
// It exists because two consumers disagreeing about this is a security bug, not
// a cosmetic one. The one-time-code rate limiter buckets by address; if it
// normalizes differently from the identity lookup, an attacker splits one
// victim's budget across arbitrarily many buckets, and the tier built to stop
// grinding a known victim stops working.
//
// strings.TrimSpace is NOT enough on its own, which is the whole reason this
// function is not a one-liner at each call site. It strips what unicode.IsSpace
// reports, and the zero-width and format characters are not in that set —
// U+200B, U+FEFF and U+2060 all survive it while the address validator accepts
// them, so an address with a trailing U+200B passes validation and keys its own
// bucket.
// Verified against the pinned validator rather than assumed.
//
// Case folding is done here as well, so a caller need not remember that the
// user store compares LOWER(email) in SQL.
func Normalize(s string) string {
	return strings.ToLower(strings.TrimFunc(s, isTrimmable))
}

// IsCanonical reports whether s survives normalization unchanged.
//
// Callers use it to REJECT rather than to silently repair: an address carrying
// an invisible character is not a typo a user made, and quietly accepting one
// means the audited value differs from what any operator would search for.
// Normalizing without this guard would still leave a caller able to write
// arbitrary variants into the audit trail on paths that record the claimed
// address.
func IsCanonical(s string) bool {
	return Normalize(s) == s
}

// isTrimmable reports whether r is whitespace or an invisible formatting
// character. The explicit runes are the ones unicode.IsSpace omits but that
// address validators accept: zero-width space, zero-width no-break space (BOM)
// and word joiner.
func isTrimmable(r rune) bool {
	switch r {
	case '\u200b', '\ufeff', '\u2060':
		return true
	default:
		return unicode.IsSpace(r) || unicode.Is(unicode.Cf, r)
	}
}

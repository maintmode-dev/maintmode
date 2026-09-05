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

// isTrimmable reports whether r is whitespace or an invisible formatting
// character.
//
// unicode.IsSpace alone is not enough, and that is the whole reason this is a
// function rather than a strings.TrimSpace at each call site: U+200B, U+FEFF and
// U+2060 are not space by that definition, yet address validators accept them,
// so a zero-width suffix used to key its own rate-limit bucket for the same
// victim. They are category Cf, which is why one predicate covers all three
// without naming them.
func isTrimmable(r rune) bool {
	return unicode.IsSpace(r) || unicode.Is(unicode.Cf, r)
}

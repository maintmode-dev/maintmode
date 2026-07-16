package entity

import (
	"strings"
	"time"
)

// CanonicalTimezone reports whether tz is an acceptable timezone preference and
// returns the value to persist.
//
// The backend is only a store here: the frontend parses and applies the zone
// (via Intl / @date-fns/tz), so this does not canonicalize casing — it just
// decides whether a value is a real user zone.
//
// Returns:
//   - ("", true)   for nil / empty / whitespace  → reset to auto-detect (not an error);
//   - (name, true) for a valid IANA identifier    → store the trimmed name;
//   - ("", false)  for anything else              → caller maps to a validation error.
//
// "Local" is rejected even though time.LoadLocation accepts it: it resolves to
// the server's machine zone, not the user's choice. Only the exact "Local" is
// special-cased — other-case spellings ("local", "LOCAL") already fail
// LoadLocation. time.LoadLocation is safe to call on untrusted input: Go rejects
// names containing ".." or a leading slash before any filesystem access, so
// there is no path-traversal surface.
func CanonicalTimezone(tz *string) (string, bool) {
	if tz == nil {
		return "", true
	}

	name := strings.TrimSpace(*tz)
	if name == "" {
		return "", true
	}

	if name == "Local" {
		return "", false
	}

	if _, err := time.LoadLocation(name); err != nil {
		return "", false
	}

	return name, true
}

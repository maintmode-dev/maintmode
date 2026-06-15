package auth

import "net/url"

// defaultOriginalURI is the post-login landing path used when no (or an unsafe)
// original_uri is supplied.
const defaultOriginalURI = "/"

// safeOriginalURI reduces the caller-supplied original_uri to a path on our own
// origin. The value travels through the OAuth flow and is handed back to the
// frontend as a client-side navigation target (JSON callback), so a raw
// attacker-controlled value is an open-redirect / phishing vector.
//
// It resolves the input against a fixed "/" base and keeps only the resulting
// path and query — any scheme or host the caller smuggled in ("//evil",
// "http://evil") is dropped by construction, and ".." / backslashes are
// normalized away. The origin can therefore never be anything but ours. Falls
// back to "/" when nothing usable remains, rather than erroring — a bad
// navigation hint must not break sign-in.
func safeOriginalURI(raw string) string {
	ref, err := url.Parse(raw)
	if err != nil {
		return defaultOriginalURI
	}

	// Resolve against a "/" base like a browser would: normalizes ".." and roots
	// a relative path. (ResolveReference still carries over any scheme/host the
	// caller smuggled in, so we strip those next.)
	base := &url.URL{Path: "/"}
	resolved := base.ResolveReference(ref)

	// Keep only path + query — deliberately dropping Scheme and Host — so the
	// result can only ever be on our own origin.
	sameOrigin := &url.URL{
		Path:     resolved.Path,
		RawQuery: resolved.RawQuery,
	}

	safe := sameOrigin.String()
	if safe == "" {
		return defaultOriginalURI
	}

	return safe
}

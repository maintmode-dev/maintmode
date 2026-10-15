package xvalidation

import (
	"fmt"
	"net/url"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"
)

// maxLinkURLLength bounds a stored link. The value is ASCII by the time the
// limit matters, so bytes and characters are the same count.
const maxLinkURLLength = 2048

// HTTPLinkURL accepts a link a person will click from a chat message or a web
// page: an absolute http(s) URL that cannot be disguised and carries no
// credentials.
//
// It differs from HTTPSURL on purpose. HTTPSURL guards URLs the SERVER fetches,
// so it refuses plain http and internal hosts. This rule guards URLs a PERSON
// opens, typically a self-hosted tracker on an internal network, so both are
// legitimate here, and the threat is instead a link that reads as one place
// and leads to another.
//
// The checks run in a fixed order and the first failure wins, so every input
// maps to exactly one code:
//   - ASCII-only, printable bytes: refuses whitespace, control characters,
//     bidi overrides and zero-width characters (which reorder or hide what the
//     reader sees) and raw IDN hosts (homographs of a real host).
//   - none of < > | ": Slack posts with escaping off and reads <url|text> as a
//     link whose visible text is chosen by the author.
//   - the decoded host is ASCII too: a percent-encoded host passes the byte
//     check yet names a non-ASCII host.
//   - no userinfo: the value is stored in cleartext and broadcast to chats.
//
// The value is never echoed in a message: a URL can carry a token, and errors
// reach logs and API responses.
//
// A nil pointer and "" pass: whether a link is required is the caller's call.
func HTTPLinkURL(value any) error {
	var raw string
	switch v := value.(type) {
	case string:
		raw = v
	case *string:
		if v == nil {
			return nil
		}
		raw = *v
	default:
		return fmt.Errorf("expected a url string, got %T", value)
	}

	if raw == "" {
		return nil
	}

	if len(raw) > maxLinkURLLength {
		return validation.NewError("validation_url_too_long",
			fmt.Sprintf("must be at most %d characters", maxLinkURLLength))
	}

	if !isPrintableASCII(raw) {
		return validation.NewError("validation_url_not_ascii",
			"must contain only printable ASCII characters, without spaces")
	}

	if strings.ContainsAny(raw, `<>|"`) {
		return validation.NewError("validation_url_forbidden_char",
			`must not contain <, >, | or "`)
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		// The parser's message quotes the raw input; report a fixed one instead.
		return validation.NewError("validation_url_invalid", "must be a valid url")
	}

	if parsed.Hostname() == "" {
		return validation.NewError("validation_url_not_absolute", "must be an absolute url")
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return validation.NewError("validation_url_not_http", "must use http or https")
	}

	if !isPrintableASCII(parsed.Hostname()) {
		return validation.NewError("validation_url_not_ascii",
			"must name the host in ASCII (punycode for international domains)")
	}

	if parsed.User != nil {
		return validation.NewError("validation_url_has_userinfo",
			"must not carry credentials in the url")
	}

	return nil
}

func isPrintableASCII(s string) bool {
	for i := range len(s) {
		if s[i] < 0x21 || s[i] > 0x7e {
			return false
		}
	}

	return true
}

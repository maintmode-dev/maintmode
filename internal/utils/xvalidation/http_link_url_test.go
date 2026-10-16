package xvalidation_test

import (
	"errors"
	"strings"
	"testing"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/utils/xvalidation"
)

// Each refused input maps to exactly one code: the checks run in a fixed order
// and the first failure wins, so a URL that is both too long and non-ASCII
// reports the length. The table pins that order.
func TestHTTPLinkURL_Refuses(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		raw  string
		code string
	}{
		"longer than 2048 bytes": {"https://x.io/" + strings.Repeat("a", 2048), "validation_url_too_long"},
		"inner space":            {"https://x.io/a b", "validation_url_not_ascii"},
		"inner control char":     {"https://x.io/a\tb", "validation_url_not_ascii"},
		"bidi override":          {"https://x.io/\u202etxt", "validation_url_not_ascii"},
		"zero-width space":       {"https://x.io/a\u200bb", "validation_url_not_ascii"},
		"raw non-ASCII host":     {"https://рф.com/t", "validation_url_not_ascii"},
		"less-than":              {"https://x.io/<a", "validation_url_forbidden_char"},
		"greater-than":           {"https://x.io/a>", "validation_url_forbidden_char"},
		"pipe":                   {"https://x.io/a|Open", "validation_url_forbidden_char"},
		"double quote":           {`https://x.io/"a`, "validation_url_forbidden_char"},
		"unparseable port":       {"https://x.io:abc/", "validation_url_invalid"},
		"relative":               {"jira/ABC-1", "validation_url_not_absolute"},
		"scheme without host":    {"https://", "validation_url_not_absolute"},
		"port without host":      {"https://:443/x", "validation_url_not_absolute"},
		"ftp scheme":             {"ftp://x.io/f", "validation_url_not_http"},
		"javascript scheme":      {"javascript://x.io/%0aalert(1)", "validation_url_not_http"},
		"percent-encoded host":   {"https://%D1%80%D1%84.com/t", "validation_url_not_ascii"},
		"user and password":      {"https://u:p@x.io", "validation_url_has_userinfo"},
		"empty userinfo":         {"https://@x.io", "validation_url_has_userinfo"},
		"password alone":         {"https://:p@x.io", "validation_url_has_userinfo"},
		// Inputs failing several checks report the earliest one.
		"too long and non-ASCII":    {"https://x.io/" + strings.Repeat("я", 1100), "validation_url_too_long"},
		"space and forbidden char":  {"https://x.io/ <", "validation_url_not_ascii"},
		"no host and wrong scheme":  {"ftp:///f", "validation_url_not_absolute"},
		"wrong scheme and userinfo": {"ftp://u:p@x.io/f", "validation_url_not_http"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := xvalidation.HTTPLinkURL(tc.raw)
			require.Error(t, err)

			var vErr validation.Error
			require.True(t, errors.As(err, &vErr), "want a validation.Error, got %T", err)
			require.Equal(t, tc.code, vErr.Code())
			// Not every refused input contains x.io (jira/ABC-1, https://), so
			// the whole value is checked too.
			require.NotContains(t, err.Error(), "x.io",
				"the message must not echo the value it refuses")
			require.NotContains(t, err.Error(), tc.raw,
				"the message must not echo the value it refuses")
		})
	}
}

func TestHTTPLinkURL_Accepts(t *testing.T) {
	t.Parallel()

	for name, raw := range map[string]string{
		"empty":                      "",
		"public https":               "https://linear.app/team/issue/ENG-1",
		"plain http internal host":   "http://jira.internal/browse/X-1",
		"private address":            "https://10.0.0.5/issue/1",
		"query with entity-like key": "https://x.io/a?b=1&not=2",
		"punycode host":              "https://xn--80ak6aa92e.com/t/1",
		"exactly 2048 bytes":         "https://x.io/" + strings.Repeat("a", 2048-len("https://x.io/")),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			require.NoError(t, xvalidation.HTTPLinkURL(raw))
		})
	}

	t.Run("pointer to a valid value", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, xvalidation.HTTPLinkURL(lo.ToPtr("https://x.io/t/1")))
	})

	t.Run("nil pointer", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, xvalidation.HTTPLinkURL((*string)(nil)))
	})
}

// The rule is reached through a *string on the update path, so a pointer must
// be judged by what it points at, not passed through as "not a string".
func TestHTTPLinkURL_JudgesPointee(t *testing.T) {
	t.Parallel()

	require.Error(t, xvalidation.HTTPLinkURL(lo.ToPtr("ftp://x.io/f")))
}

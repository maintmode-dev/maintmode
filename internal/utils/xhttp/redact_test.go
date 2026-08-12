package xhttp

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSafeURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			// The verbatim expectation from safeURL's doc comment. Asserting the
			// exact string — rather than "the secret is absent" — is what stops an
			// implementation that drops the query wholesale from passing.
			name: "telegram token in path and query",
			raw:  "https://api.telegram.org/bot" + testToken + "/sendMessage?access_token=" + testToken + "&x=1",
			want: "https://api.telegram.org/[REDACTED]/[REDACTED]?access_token=[REDACTED]&x=[REDACTED]",
		},
		{
			name: "no query",
			raw:  "https://console.example.com/cloud/v1/instances/heartbeat",
			want: "https://console.example.com/[REDACTED]/[REDACTED]/[REDACTED]/[REDACTED]",
		},
		{
			name: "root path",
			raw:  "https://example.com/",
			want: "https://example.com/",
		},
		{
			name: "no path at all",
			raw:  "https://example.com",
			want: "https://example.com",
		},
		{
			name: "query only",
			raw:  "https://example.com/?token=secret",
			want: "https://example.com/?token=[REDACTED]",
		},
		{
			// Segment count is preserved, so a doubled slash must not silently
			// become a single one: that would be inventing structure.
			name: "doubled slash",
			raw:  "https://example.com//a//b",
			want: "https://example.com//[REDACTED]//[REDACTED]",
		},
		{
			// Stable ordering: map iteration is not, and a flaky log format would
			// be a poor trade for a security fix.
			name: "query keys are sorted",
			raw:  "https://example.com/x?zebra=1&alpha=2&mid=3",
			want: "https://example.com/[REDACTED]?alpha=[REDACTED]&mid=[REDACTED]&zebra=[REDACTED]",
		},
		{
			name: "userinfo is dropped entirely",
			raw:  "https://user:pass@example.com/x",
			want: "https://example.com/[REDACTED]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			u, err := url.Parse(tt.raw)
			require.NoError(t, err)

			require.Equal(t, tt.want, safeURL(u))
		})
	}
}

// A nil URL must not panic. Logging is a side effect; it may never be the reason a
// request dies.
func TestSafeURLNil(t *testing.T) {
	t.Parallel()

	require.Equal(t, "", safeURL(nil))
}

func TestRedactHeaders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   http.Header
		want map[string]string
	}{
		{
			name: "nil header",
			in:   nil,
			want: nil,
		},
		{
			name: "empty header",
			in:   http.Header{},
			want: nil,
		},
		{
			name: "authorization is redacted, content-type survives",
			in: http.Header{
				"Authorization": []string{"Bearer " + testToken},
				"Content-Type":  []string{"application/json"},
			},
			want: map[string]string{
				"Authorization": redacted,
				"Content-Type":  "application/json",
			},
		},
		{
			name: "every blocklisted name, both directions",
			in: http.Header{
				"Authorization":       []string{"Bearer x"},
				"Proxy-Authorization": []string{"Basic x"},
				"Cookie":              []string{"session=x"},
				"Set-Cookie":          []string{"session=x"},
				"Www-Authenticate":    []string{`Bearer realm="x"`},
				"Proxy-Authenticate":  []string{"Basic realm=x"},
			},
			want: map[string]string{
				"Authorization":       redacted,
				"Proxy-Authorization": redacted,
				"Cookie":              redacted,
				"Set-Cookie":          redacted,
				"Www-Authenticate":    redacted,
				"Proxy-Authenticate":  redacted,
			},
		},
		{
			// A caller assigning the map key directly bypasses the canonicalization
			// Set/Add would have done. A raw map-key comparison misses this, and the
			// miss is silent.
			name: "non-canonical key is still matched",
			in: http.Header{
				"authorization": []string{"Bearer " + testToken},
			},
			want: map[string]string{
				"Authorization": redacted,
			},
		},
		{
			name: "multi-value non-sensitive header is joined",
			in: http.Header{
				"Accept": []string{"application/json", "text/plain"},
			},
			want: map[string]string{
				"Accept": "application/json, text/plain",
			},
		},
		{
			// Multi-value Set-Cookie must not leak the second cookie through the
			// join: the whole entry is replaced, not each value.
			name: "multi-value sensitive header is replaced once",
			in: http.Header{
				"Set-Cookie": []string{"a=1", "b=2"},
			},
			want: map[string]string{
				"Set-Cookie": redacted,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, redactHeaders(tt.in))
		})
	}
}

// The non-mutation guarantee, tested directly on the pure function.
//
// http.Client re-enters RoundTrip with the SAME *http.Request on retries and
// redirects, so a mutating redactor corrupts the header on the second attempt even
// without concurrency. And a stripped Authorization surfaces as a runtime 401, not
// as a red test — which is exactly why this assertion exists.
func TestRedactHeadersDoesNotMutate(t *testing.T) {
	t.Parallel()

	header := http.Header{}
	header.Set("Authorization", "Bearer "+testToken)
	header.Add("Set-Cookie", "a=1")
	header.Add("Set-Cookie", "b=2")

	_ = redactHeaders(header)

	require.Equal(t, "Bearer "+testToken, header.Get("Authorization"))
	require.Equal(t, []string{"a=1", "b=2"}, header.Values("Set-Cookie"))
}

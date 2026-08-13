package xsanitize

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

const secret = "S3CRETVALUE"

// TestSanitizeHeadersRedactsSensitive covers both directions of the blocklist:
// request-side names and response-side ones are one union, applied to both.
func TestSanitizeHeadersRedactsSensitive(t *testing.T) {
	t.Parallel()

	in := http.Header{
		"Authorization":       []string{"Bearer " + secret},
		"Proxy-Authorization": []string{secret},
		"Cookie":              []string{"session=" + secret},
		"Set-Cookie":          []string{"session=" + secret},
		"Www-Authenticate":    []string{"Basic realm=" + secret},
		"Proxy-Authenticate":  []string{secret},
		"Content-Type":        []string{"application/json"},
	}

	got := New().SanitizeHeaders(in)

	for _, name := range []string{
		"Authorization", "Proxy-Authorization", "Cookie",
		"Set-Cookie", "Www-Authenticate", "Proxy-Authenticate",
	} {
		assert.Equal(t, []string{redacted}, got[name], name)
	}

	assert.Equal(t, []string{"application/json"}, got["Content-Type"],
		"non-sensitive headers survive, or the log loses its diagnostics")
}

// A caller that writes req.Header["authorization"] directly bypasses the
// canonicalization Set/Add would have done. A raw map lookup would miss it.
func TestSanitizeHeadersCanonicalizesNames(t *testing.T) {
	t.Parallel()

	got := New().SanitizeHeaders(http.Header{"authorization": []string{secret}})

	assert.Equal(t, []string{redacted}, got["Authorization"])
	assert.NotContains(t, got, "authorization")
}

// The redactor must never mutate the caller's header map: http.Client re-enters
// RoundTrip with the same *http.Request on retries and redirects, so a mutating
// one corrupts the second attempt — stripping the Authorization header from a
// request that is about to be sent again.
func TestSanitizeHeadersDoesNotMutateInput(t *testing.T) {
	t.Parallel()

	values := []string{"Bearer " + secret}
	in := http.Header{"Authorization": values}

	_ = New().SanitizeHeaders(in)

	assert.Equal(t, "Bearer "+secret, in.Get("Authorization"), "the live request must be untouched")
	assert.Equal(t, "Bearer "+secret, values[0], "the backing slice must be untouched too")
}

func TestSanitizeHeadersEmpty(t *testing.T) {
	t.Parallel()

	assert.Nil(t, New().SanitizeHeaders(nil))
	assert.Nil(t, New().SanitizeHeaders(http.Header{}))
}

func TestSanitizeURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			// The point of the current policy: a readable route. Masking every
			// segment would render this as four identical markers.
			name: "path survives",
			in:   "https://console.example.com/cloud/v1/instances/heartbeat",
			want: "https://console.example.com/cloud/v1/instances/heartbeat",
		},
		{
			name: "query survives",
			in:   "https://api.example.com/api/chat.postMessage?page=2",
			want: "https://api.example.com/api/chat.postMessage?page=2",
		},
		{
			// "user:pass@" is a credential by definition, so it is the one part
			// this policy removes.
			name: "userinfo is dropped",
			in:   "https://user:" + secret + "@api.example.com/v1/thing",
			want: "https://api.example.com/v1/thing",
		},
		{
			name: "empty stays empty",
			in:   "",
			want: "",
		},
		{
			// Failing open would print whatever could not be parsed.
			name: "unparseable yields the marker",
			in:   "://" + secret,
			want: redacted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, New().SanitizeURL(tt.in))
		})
	}
}

// Bodies are passed through: their shape is arbitrary, so a blocklist cannot
// help, and body logging is off unless explicitly enabled.
func TestSanitizeBodyPassesThrough(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []byte(secret), New().SanitizeBody([]byte(secret)))
}

package xecho

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// secret stands in for a token. Assertions compare against it exactly, so it
// must never appear in an expected value by accident.
const secret = "S3CRETVALUE"

// TestExtractBearerToken pins the parse rules of the Authorization header.
//
// The table is built around the one property that matters at a call site: the
// return value is either the exact token or the empty string, and the empty
// string must mean "no usable credential". A permissive implementation — one
// that strips any prefix, trims whitespace, or matches case-insensitively —
// would hand a caller a token that the peer never sent under the scheme it
// expects, so each malformed row asserts Empty rather than merely "not equal".
func TestExtractBearerToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		header string
		set    bool
		want   string
	}{
		{
			name:   "well formed bearer header yields the raw token",
			header: "Bearer " + secret,
			set:    true,
			want:   secret,
		},
		{
			name: "absent header yields empty",
			set:  false,
			want: "",
		},
		{
			name:   "empty header yields empty",
			header: "",
			set:    true,
			want:   "",
		},
		{
			name:   "scheme with no token yields empty token",
			header: "Bearer ",
			set:    true,
			want:   "",
		},
		{
			name: "scheme without the separating space is not a bearer header",
			// CutPrefix requires the trailing space, so this is rejected whole
			// rather than yielding the token with a leading rune eaten.
			header: "Bearer" + secret,
			set:    true,
			want:   "",
		},
		{
			name:   "a different scheme yields empty",
			header: "Basic " + secret,
			set:    true,
			want:   "",
		},
		{
			name: "lowercase scheme is not accepted",
			// Documents the current contract: the match is case-sensitive.
			header: "bearer " + secret,
			set:    true,
			want:   "",
		},
		{
			name: "leading whitespace before the scheme yields empty",
			// No trimming happens, so a padded header is not silently accepted.
			header: " Bearer " + secret,
			set:    true,
			want:   "",
		},
		{
			name: "inner spacing and padding are preserved verbatim",
			// Everything after the first "Bearer " is the token, untouched.
			// A defensive TrimSpace would break this expectation.
			header: "Bearer  " + secret + " ",
			set:    true,
			want:   " " + secret + " ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://example.invalid", http.NoBody)
			require.NoError(t, err)

			if tt.set {
				req.Header.Set("Authorization", tt.header)
			}

			assert.Equal(t, tt.want, ExtractBearerToken(req))
		})
	}
}

// TestSetBearerToken asserts the exact wire form of the header the setter writes.
//
// Comparing the whole header value, not just "contains the token": a setter that
// wrote the bare token, used a different scheme, or dropped the space would still
// satisfy a Contains check while producing a header no server would accept.
func TestSetBearerToken(t *testing.T) {
	t.Parallel()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://example.invalid", http.NoBody)
	require.NoError(t, err)

	SetBearerToken(req, secret)

	assert.Equal(t, "Bearer "+secret, req.Header.Get("Authorization"))
}

// TestSetBearerTokenReplacesExistingHeader pins Set-not-Add semantics.
//
// http.Header supports multiple values per key. If the implementation ever used
// Add, the stale credential would survive alongside the new one and the plain
// Get above would still return the first value — so this asserts on the whole
// value slice, which is the only place that distinction is visible.
func TestSetBearerTokenReplacesExistingHeader(t *testing.T) {
	t.Parallel()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://example.invalid", http.NoBody)
	require.NoError(t, err)

	req.Header.Set("Authorization", "Bearer stale-token")
	SetBearerToken(req, secret)

	assert.Equal(t, []string{"Bearer " + secret}, req.Header.Values("Authorization"),
		"the previous credential must be replaced, not appended")
	assert.NotContains(t, req.Header.Get("Authorization"), "stale-token")
}

// TestSetBearerTokenRoundTripsThroughExtract ties the two halves together.
//
// Each function alone can only be checked against a literal this test file
// chose; together they pin the property the pair exists for. It would catch a
// coordinated change of the scheme constant that kept both sides self-consistent
// but silently altered the wire format — which the literal assertions above
// would flag, while a caller using only this pair would not.
func TestSetBearerTokenRoundTripsThroughExtract(t *testing.T) {
	t.Parallel()

	tokens := []string{
		secret,
		"a",
		"token.with.dots-and_dashes",
		"eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.sig",
	}

	for _, token := range tokens {
		t.Run(token, func(t *testing.T) {
			t.Parallel()

			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://example.invalid", http.NoBody)
			require.NoError(t, err)

			SetBearerToken(req, token)

			assert.Equal(t, token, ExtractBearerToken(req))
		})
	}
}

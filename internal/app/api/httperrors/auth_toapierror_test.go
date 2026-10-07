package httperrors

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
)

// A refused token and an unreachable IdP answer with the sentinel's own text.
// The wrapped chain is what the verifier's dependencies produced -- go-oidc
// quotes the body of a failed JWKS fetch, and the fetch went wherever the
// provider's document pointed -- so reflecting it hands the caller that body.
func TestToAPIError_AuthFailuresDoNotReflectTheChain(t *testing.T) {
	t.Parallel()

	// Shaped like the real chain: verify.go wraps the library's error under the
	// sentinel, and the library's error carries the far end's response.
	const leaked = "SECRET-INTERNAL-RESPONSE"
	upstream := fmt.Errorf("failed to verify signature: fetching keys oidc: get keys failed: 200 OK %s", leaked)

	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantMsg    string
	}{
		{
			"invalid token",
			fmt.Errorf("%w: %w", apperr.ErrInvalidAccessToken, upstream),
			http.StatusUnauthorized, apperr.ErrInvalidAccessToken.Error(),
		},
		{
			"expired token",
			fmt.Errorf("%w: %w", apperr.ErrTokenExpired, upstream),
			http.StatusUnauthorized, apperr.ErrTokenExpired.Error(),
		},
		{
			"provider unavailable",
			fmt.Errorf("%w: resolve discovery for custom: %w", apperr.ErrAuthUnavailable, upstream),
			http.StatusServiceUnavailable, apperr.ErrAuthUnavailable.Error(),
		},
	}

	e := echo.New()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			c := e.NewContext(httptest.NewRequest(http.MethodPost, "/", http.NoBody), rec)

			require.NoError(t, ToAPIError(c, "auth.connectProvider", tc.err))
			require.Equal(t, tc.wantStatus, rec.Code)
			require.NotContains(t, rec.Body.String(), leaked)

			var resp ErrorResponse
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
			require.Equal(t, tc.wantMsg, resp.Message)
		})
	}
}

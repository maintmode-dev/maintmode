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

// ToAPIError is the mapper the role/invite handlers actually call. This pins the
// seats-cap contract through that real path: 403 with the stable
// seats_limit_exceeded code. Unlike the sibling suspend/signup cases (which
// return a fixed generic message), the seats case surfaces err.Error() verbatim
// — the "all N of M seats are in use" numbers live in the message and the FE
// renders them. A regression that swapped it to a fixed string, or reordered it
// after the generic ErrForbidden case, would silently break that contract; this
// test catches it. The wrapped variant models how the error actually arrives
// from the service (fmt.Errorf("all %d of %d seats are in use: %w", ...)).
func TestToAPIError_SeatsLimitExceeded(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		err     error
		wantMsg string
	}{
		{"sentinel", apperr.ErrSeatsLimitExceeded, "seats limit exceeded"},
		{
			"wrapped with counts",
			fmt.Errorf("all %d of %d seats are in use: %w", 4, 4, apperr.ErrSeatsLimitExceeded),
			"all 4 of 4 seats are in use: seats limit exceeded",
		},
	}

	e := echo.New()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			c := e.NewContext(httptest.NewRequest(http.MethodPost, "/", http.NoBody), rec)

			require.NoError(t, ToAPIError(c, "user.assignRoles", tc.err))
			require.Equal(t, http.StatusForbidden, rec.Code)

			var resp ErrorResponse
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
			require.Equal(t, string(ErrSeatsLimitExceeded), resp.Code)
			// The numbers must survive to the body verbatim — the code is the
			// machine anchor, the message carries the human-readable counts.
			require.Equal(t, tc.wantMsg, resp.Message)
		})
	}
}

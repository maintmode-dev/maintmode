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

// Linking a provider account that is already linked -- to the caller, or to
// somebody else -- must answer identically. A different message would tell the
// caller that a provider account they hold belongs to another person here,
// which the dance's link_conflict code already refuses to say.
func TestToAPIError_LinkConflictsAreIndistinguishable(t *testing.T) {
	t.Parallel()

	e := echo.New()
	answer := func(err error) (int, ErrorResponse) {
		rec := httptest.NewRecorder()
		c := e.NewContext(httptest.NewRequest(http.MethodPost, "/", http.NoBody), rec)

		require.NoError(t, ToAPIError(c, "connect provider", fmt.Errorf("link identity: %w", err)))

		var resp ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

		return rec.Code, resp
	}

	mineStatus, mine := answer(apperr.ErrProviderAlreadyConnected)
	theirsStatus, theirs := answer(apperr.ErrProviderLinkedToAnotherUser)

	require.Equal(t, http.StatusConflict, mineStatus)
	require.Equal(t, mineStatus, theirsStatus)
	require.Equal(t, mine, theirs)
}

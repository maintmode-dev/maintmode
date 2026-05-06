package roles

import (
	"net/http"
	"testing"

	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/require"

	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/roles/models"
	testjsonudils "github.com/ruko1202/maintmode/test/utils/json"
)

func TestAvailableRoles(t *testing.T) {
	t.Parallel()

	impl := initImpl(t)

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)

		err := impl.AvailableRoles(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rec.Code)

		resp := testjsonudils.JSONToAny[apimodels.ListRolesResponse](t, rec.Body)
		require.Equal(t, apimodels.AllRoles, resp.Roles)
	})
}

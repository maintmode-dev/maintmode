package roles

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/require"

	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/roles/models"
	testjsonudils "github.com/ruko1202/maintmode/test/utils/json"
)

func TestListRoles(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t)

	t.Run("invalid uuid", func(t *testing.T) {
		t.Parallel()

		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{
			{Name: "id", Value: "invalid-uuid"},
		})

		err := impl.ListRoles(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("ok returns roles for user", func(t *testing.T) {
		t.Parallel()

		user := createUser(ctx, t, impl, apimodels.RoleEditor)

		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{
			{Name: "id", Value: user.ID.String()},
		})

		err := impl.ListRoles(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rec.Code)

		resp := testjsonudils.JSONToAny[apimodels.ListRolesResponse](t, rec.Body)
		require.Equal(t, []apimodels.Role{apimodels.RoleEditor}, resp.Roles)
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{
			{Name: "id", Value: uuid.NewString()},
		})

		err := impl.ListRoles(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusNotFound, rec.Code)
	})
}

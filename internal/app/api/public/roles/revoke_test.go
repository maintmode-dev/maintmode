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
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
	testjsonudils "github.com/ruko1202/maintmode/test/utils/json"
)

func TestRevoke(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t)

	t.Run("ок", func(t *testing.T) {
		t.Parallel()

		target := createUser(ctx, t, impl, apimodels.RoleGuest, apimodels.RoleEditor)
		req := &apimodels.AssignRoleRequest{
			UserID: target.ID.String(),
			Role:   apimodels.RoleEditor,
		}

		c, rec := echotest.ContextConfig{
			JSONBody: testjsonudils.AnyToJSONBytes(t, req),
		}.ToContextRecorder(t)
		xecho.UserToEchoCtx(c, &entity.User{
			ID:    uuid.New(),
			Roles: []entity.Role{entity.RoleAdmin},
			Email: "admin@test.com",
		})

		err := impl.Revoke(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, rec.Code)

		c, rec = echotest.ContextConfig{}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{
			{Name: "id", Value: target.ID.String()},
		})

		err = impl.ListRoles(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rec.Code)

		resp := testjsonudils.JSONToAny[apimodels.ListRolesResponse](t, rec.Body)
		require.Equal(t, []apimodels.Role{apimodels.RoleGuest}, resp.Roles)
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()

		for _, tc := range []struct {
			name          string
			req           *apimodels.RevokeRoleRequest
			expStatusCode int
		}{
			{
				name: "missing user_id",
				req: &apimodels.RevokeRoleRequest{
					Role: apimodels.RoleEditor,
				},
				expStatusCode: http.StatusBadRequest,
			}, {
				name: "missing role",
				req: &apimodels.RevokeRoleRequest{
					UserID: uuid.New().String(),
				},
				expStatusCode: http.StatusBadRequest,
			}, {
				name: "no actor in context",
				req: &apimodels.RevokeRoleRequest{
					UserID: uuid.New().String(),
					Role:   apimodels.RoleEditor,
				},
				expStatusCode: http.StatusBadRequest,
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				c, rec := echotest.ContextConfig{
					JSONBody: testjsonudils.AnyToJSONBytes(t, tc.req),
				}.ToContextRecorder(t)

				err := impl.Revoke(c)
				require.NoError(t, err)
				require.Equal(t, tc.expStatusCode, rec.Code)
			})
		}
	})
}

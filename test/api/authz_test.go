//go:build api

package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/test/api/client/client/audit"
	"github.com/ruko1202/maintmode/test/api/client/client/resources"
	"github.com/ruko1202/maintmode/test/api/client/client/roles"
	"github.com/ruko1202/maintmode/test/api/client/models"
)

func TestMaintmodeAPIAuth_MissingToken(t *testing.T) {
	ctx := context.Background()
	apiClient := setupMaintmodeTestClientWithoutAuth()

	params := resources.NewGetAPIV1ResourcesParams().WithContext(ctx)

	_, err := apiClient.Resources.GetAPIV1Resources(params, nil)
	require.Error(t, err)
	require.Equal(t, http.StatusUnauthorized, extractErrorCode(t, err))
}

func TestMaintmodeAPIAuth_InvalidToken(t *testing.T) {
	ctx := context.Background()
	apiClient := setupMaintmodeTestClientWithToken("invalid")

	params := resources.NewGetAPIV1ResourcesParams().WithContext(ctx)

	_, err := apiClient.Resources.GetAPIV1Resources(params, nil)
	require.Error(t, err)
	require.Equal(t, http.StatusUnauthorized, extractErrorCode(t, err))
}

func TestMaintmodeAPIRBAC_GuestReadAllowed(t *testing.T) {
	ctx := context.Background()
	apiClient := setupMaintmodeTestClientWithRoles(entity.RoleGuest)

	params := resources.NewGetAPIV1ResourcesParams().WithContext(ctx)

	resp, err := apiClient.Resources.GetAPIV1Resources(params, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestMaintmodeAPIRBAC_GuestMutationForbidden(t *testing.T) {
	ctx := context.Background()
	apiClient := setupMaintmodeTestClientWithRoles(entity.RoleGuest)

	params := resources.NewPostAPIV1ResourceCreateParams().
		WithContext(ctx).
		WithRequest(&models.ApismodelsCreateResourceRequest{
			Name:        "forbidden resource",
			Description: "guest must not create resources",
		})

	_, err := apiClient.Resources.PostAPIV1ResourceCreate(params, nil)
	require.Error(t, err)
	require.Equal(t, http.StatusForbidden, extractErrorCode(t, err))
}

func TestAuthAPIRBAC_GuestCanReadRoles(t *testing.T) {
	ctx := context.Background()
	apiClient := setupAuthTestClientWithRoles(entity.RoleGuest)

	params := roles.NewGetAPIV1RolesParams().WithContext(ctx)

	resp, err := apiClient.Roles.GetAPIV1Roles(params, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestAuthAPIRBAC_GuestAuditForbidden(t *testing.T) {
	ctx := context.Background()
	apiClient := setupAuthTestClientWithRoles(entity.RoleGuest)
	limit := int64(1)

	params := audit.NewGetAPIV1AuditLogParams().
		WithContext(ctx).
		WithLimit(&limit)

	_, err := apiClient.Audit.GetAPIV1AuditLog(params, nil)
	require.Error(t, err)
	require.Equal(t, http.StatusForbidden, extractErrorCode(t, err))
}

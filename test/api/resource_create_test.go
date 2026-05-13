package api

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/utils/xuuid"
	"github.com/ruko1202/maintmode/test/api/client/client/resources"
	"github.com/ruko1202/maintmode/test/api/client/models"
)

func TestReroucesAPI_Create(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	apiClient := setupMaintmodeTestClient()

	for _, tc := range []struct {
		name string
		req  *models.ApismodelsCreateResourceRequest
	}{
		{
			name: "empty external id",
			req: &models.ApismodelsCreateResourceRequest{
				Name:        fmt.Sprintf("test name: %s [%s]", t.Name(), xuuid.NewString()),
				Description: "This is a test resource created via API tests",
			},
		}, {
			name: "with external id",
			req: &models.ApismodelsCreateResourceRequest{
				Name:        fmt.Sprintf("test name: %s [%s]", t.Name(), xuuid.NewString()),
				Description: "This is a test resource created via API tests",
				ExternalID:  xuuid.NewString(),
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			params := resources.NewPostAPIV1ResourceCreateParams().
				WithContext(ctx).
				WithRequest(tc.req)

			resp, err := apiClient.Resources.PostAPIV1ResourceCreate(params, nil)
			require.NoError(t, err)
			require.NotNil(t, resp)

			payload := resp.Payload
			require.NotEmpty(t, payload.ID)
			require.Equal(t, tc.req.Name, payload.Name)
			require.Equal(t, tc.req.Description, payload.Description)
			require.Equal(t, tc.req.ExternalID, payload.ExternalID)
			require.False(t, payload.CreatedAt.IsZero())
		})
	}
}

func TestReroucesAPI_Create_IdempotentCaseInsensitive(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	apiClient := setupMaintmodeTestClient()

	name := fmt.Sprintf("test name: %s [%s]", t.Name(), xuuid.NewString())

	first := resources.NewPostAPIV1ResourceCreateParams().
		WithContext(ctx).
		WithRequest(&models.ApismodelsCreateResourceRequest{
			Name:        name,
			Description: "first create",
		})
	firstResp, err := apiClient.Resources.PostAPIV1ResourceCreate(first, nil)
	require.NoError(t, err)
	require.NotEmpty(t, firstResp.Payload.ID)

	second := resources.NewPostAPIV1ResourceCreateParams().
		WithContext(ctx).
		WithRequest(&models.ApismodelsCreateResourceRequest{
			Name:        strings.ToUpper(name),
			Description: "second create with different case",
		})
	secondResp, err := apiClient.Resources.PostAPIV1ResourceCreate(second, nil)
	require.NoError(t, err)
	require.Equal(t, firstResp.Payload.ID, secondResp.Payload.ID,
		"creating resource with a name differing only by case must return the existing resource")
}

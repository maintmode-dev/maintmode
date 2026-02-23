package api

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/utils/xuuid"
	"github.com/ruko1202/maintmode/test/api/client/client/resources"
	"github.com/ruko1202/maintmode/test/api/client/models"
)

func TestReroucesAPI_Create(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	apiClient := setupTestClient()

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

			resp, err := apiClient.Resources.PostAPIV1ResourceCreate(params)
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

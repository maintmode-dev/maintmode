package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/utils/xuuid"
	maintmodeclient "github.com/ruko1202/maintmode/test/api/client/maintmode"
)

func TestReroucesAPI_Create(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	apiClient := setupMaintmodeTestClient()

	for _, tc := range []struct {
		name string
		req  maintmodeclient.PostApiV1ResourceCreateJSONRequestBody
	}{
		{
			name: "empty external id",
			req: maintmodeclient.PostApiV1ResourceCreateJSONRequestBody{
				Name:        lo.ToPtr(fmt.Sprintf("test name: %s [%s]", t.Name(), xuuid.NewString())),
				Description: lo.ToPtr("This is a test resource created via API tests"),
			},
		}, {
			name: "with external id",
			req: maintmodeclient.PostApiV1ResourceCreateJSONRequestBody{
				Name:        lo.ToPtr(fmt.Sprintf("test name: %s [%s]", t.Name(), xuuid.NewString())),
				Description: lo.ToPtr("This is a test resource created via API tests"),
				ExternalId:  lo.ToPtr(xuuid.NewString()),
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			resp, err := apiClient.PostApiV1ResourceCreateWithResponse(ctx, tc.req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.StatusCode(), "unexpected status: %s", resp.Body)
			require.NotNil(t, resp.JSON200)

			payload := resp.JSON200
			require.NotEmpty(t, lo.FromPtr(payload.Id))
			require.Equal(t, lo.FromPtr(tc.req.Name), lo.FromPtr(payload.Name))
			require.Equal(t, lo.FromPtr(tc.req.Description), lo.FromPtr(payload.Description))
			require.Equal(t, lo.FromPtr(tc.req.ExternalId), lo.FromPtr(payload.ExternalId))
			require.False(t, lo.FromPtr(payload.CreatedAt).IsZero())
		})
	}
}

func TestReroucesAPI_Create_IdempotentCaseInsensitive(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	apiClient := setupMaintmodeTestClient()

	name := fmt.Sprintf("test name: %s [%s]", t.Name(), xuuid.NewString())

	firstResp, err := apiClient.PostApiV1ResourceCreateWithResponse(ctx, maintmodeclient.PostApiV1ResourceCreateJSONRequestBody{
		Name:        lo.ToPtr(name),
		Description: lo.ToPtr("first create"),
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, firstResp.StatusCode(), "unexpected status: %s", firstResp.Body)
	require.NotNil(t, firstResp.JSON200)
	require.NotEmpty(t, lo.FromPtr(firstResp.JSON200.Id))

	secondResp, err := apiClient.PostApiV1ResourceCreateWithResponse(ctx, maintmodeclient.PostApiV1ResourceCreateJSONRequestBody{
		Name:        lo.ToPtr(strings.ToUpper(name)),
		Description: lo.ToPtr("second create with different case"),
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, secondResp.StatusCode(), "unexpected status: %s", secondResp.Body)
	require.NotNil(t, secondResp.JSON200)
	require.Equal(t, lo.FromPtr(firstResp.JSON200.Id), lo.FromPtr(secondResp.JSON200.Id),
		"creating resource with a name differing only by case must return the existing resource")
}

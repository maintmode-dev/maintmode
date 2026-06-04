package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	maintmodeclient "github.com/ruko1202/maintmode/internal/pkg/generated/clients/maintmode"
)

func TestReroucesAPI_List(t *testing.T) {
	t.Parallel()
	ctx := ctxWithLogger(context.Background(), t)
	apiClient := setupMaintmodeTestClient()

	createdResources := lo.RepeatBy(150, func(_ int) maintmodeclient.ApimodelsResource {
		return creatResource(ctx, t, apiClient)
	})
	resource := lo.LastOrEmpty(createdResources)

	for _, tc := range []struct {
		name              string
		searchName        string
		expectedCount     int
		expectedResources []maintmodeclient.ApimodelsResource
	}{
		{
			name:              "search by name",
			searchName:        lo.FromPtr(resource.Name),
			expectedCount:     1,
			expectedResources: []maintmodeclient.ApimodelsResource{resource},
		}, {
			name:              "search all",
			searchName:        "",
			expectedCount:     100,
			expectedResources: []maintmodeclient.ApimodelsResource{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			resp, err := apiClient.GetApiV1ResourcesWithResponse(ctx, &maintmodeclient.GetApiV1ResourcesParams{
				Name: tc.searchName,
			})
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.StatusCode(), "unexpected status: %s", resp.Body)
			require.NotNil(t, resp.JSON200)

			payload := resp.JSON200

			actualResourceM := lo.SliceToMap(lo.FromPtr(payload.Resources), func(item maintmodeclient.ApimodelsResource) (string, maintmodeclient.ApimodelsResource) {
				return lo.FromPtr(item.Id), item
			})
			require.GreaterOrEqual(t, len(actualResourceM), tc.expectedCount)

			for _, expected := range tc.expectedResources {
				actual, ok := actualResourceM[lo.FromPtr(expected.Id)]
				require.True(t, ok)
				require.Equal(t, expected, actual)
			}
		})
	}
}

package api

import (
	"context"
	"testing"

	"github.com/go-openapi/strfmt"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/test/api/client/client/resources"
	"github.com/ruko1202/maintmode/test/api/client/models"
)

func TestReroucesAPI_List(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	apiClient := setupMaintmodeTestClient()

	createdResources := lo.RepeatBy(150, func(_ int) *models.ApimodelsResource {
		return creatResource(ctx, t, apiClient)
	})
	resource := lo.LastOrEmpty(createdResources)

	for _, tc := range []struct {
		name              string
		searchName        string
		expectedCount     int
		expectedResources []*models.ApimodelsResource
	}{
		{
			name:              "search by name",
			searchName:        resource.Name,
			expectedCount:     1,
			expectedResources: []*models.ApimodelsResource{resource},
		}, {
			name:              "search all",
			searchName:        "",
			expectedCount:     100,
			expectedResources: []*models.ApimodelsResource{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			params := resources.NewGetAPIV1ResourcesParams().
				WithContext(ctx).
				WithName(tc.searchName)

			resp, err := apiClient.Resources.GetAPIV1Resources(params, nil)
			require.NoError(t, err)
			require.NotNil(t, resp)

			payload := resp.Payload
			require.NotNil(t, payload)

			actualResourceM := lo.SliceToMap(payload.Resources, func(item *models.ApimodelsResource) (strfmt.UUID, *models.ApimodelsResource) {
				return strfmt.UUID(item.ID), item
			})
			require.GreaterOrEqual(t, len(actualResourceM), tc.expectedCount)

			for _, expected := range tc.expectedResources {
				actual, ok := actualResourceM[strfmt.UUID(expected.ID)]
				require.True(t, ok)
				require.Equal(t, expected, actual)
			}
		})
	}
}

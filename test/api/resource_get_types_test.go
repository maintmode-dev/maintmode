package api

import (
	"context"
	"testing"

	"github.com/go-openapi/strfmt"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/test/api/client/client/resources"
	"github.com/ruko1202/maintmode/test/api/client/models"
)

func TestReroucesAPI_GetTypes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	apiClient := setupMaintmodeTestClient()

	resource := creatResource(ctx, t, apiClient)

	params := resources.NewGetAPIV1ResourceIDTypesParams().
		WithContext(ctx).
		WithID(strfmt.UUID(resource.ID))

	resp, err := apiClient.Resources.GetAPIV1ResourceIDTypes(params)
	require.NoError(t, err)
	require.NotNil(t, resp)

	payload := resp.Payload
	require.NotEmpty(t, payload.Types)
	expected := []*models.ApismodelsResourceType{
		{
			Type: models.EntityResourceTypeService,
		}, {
			Type: models.EntityResourceTypeDatabase,
		}, {
			Type: models.EntityResourceTypeCluster,
		},
	}

	require.Equal(t, len(expected), len(payload.Types))
	require.Equal(t, expected, payload.Types)
}

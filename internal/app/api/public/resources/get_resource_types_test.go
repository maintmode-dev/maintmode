package resourcesapi

import (
	"net/http"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"

	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/resources/models"
	testjsonudils "github.com/ruko1202/maintmode/test/utils/json"
)

func TestGetResourceTypes(t *testing.T) {
	t.Parallel()

	impl := initImpl(t)

	t.Run("invalid uuid", func(t *testing.T) {
		t.Parallel()

		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{
			{Name: "id", Value: "invalid-uuid"},
		})

		err := impl.GetResourceTypes(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("ok returns types for created resource", func(t *testing.T) {
		t.Parallel()

		created := makeResource(t, impl)

		c, rec := echotest.ContextConfig{
			PathValues: echo.PathValues{
				{Name: "id", Value: created.ID.String()},
			},
		}.ToContextRecorder(t)

		err := impl.GetResourceTypes(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rec.Code)

		resp := testjsonudils.JSONToAny[apimodels.GetResourceTypesResponse](t, rec.Body)
		require.Equal(t, apimodels.ToAPIResourceTypes([]entity.ResourceType{
			entity.ResourceTypeService,
			entity.ResourceTypeDatabase,
			entity.ResourceTypeCluster,
		}), resp.Types)
	})
}

package resourcesapi

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/labstack/echo/v5/echotest"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/resources/models"
	testjsonudils "github.com/ruko1202/maintmode/test/utils/json"
)

func TestSearchResources(t *testing.T) {
	t.Parallel()

	impl := initImpl(t)

	t.Run("ok empty result", func(t *testing.T) {
		t.Parallel()

		c, rec := echotest.ContextConfig{
			QueryValues: url.Values{
				"name": {"nonexistent-resource-xyz"},
			},
		}.ToContextRecorder(t)

		err := impl.SearchResources(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rec.Code)

		resp := testjsonudils.JSONToAny[apimodels.SearchResourcesResponse](t, rec.Body)
		require.Empty(t, resp.Items)
	})

	t.Run("ok finds created resource", func(t *testing.T) {
		t.Parallel()

		resource := makeResource(t, impl)

		c, rec := echotest.ContextConfig{
			QueryValues: url.Values{
				"name": {resource.Name},
			},
		}.ToContextRecorder(t)

		err := impl.SearchResources(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rec.Code)

		resp := testjsonudils.JSONToAny[apimodels.SearchResourcesResponse](t, rec.Body)
		require.NotEmpty(t, resp.Items)

		actualResource, ok := lo.Find(resp.Items, func(item *apimodels.Resource) bool {
			return item.Name == resource.Name
		})
		require.True(t, ok)
		require.Equal(t, resource, actualResource)
	})
}

package resourcesapi

import (
	"net/http"
	"testing"

	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/require"

	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/resources/models"
	testjsonudils "github.com/ruko1202/maintmode/test/utils/json"
)

func TestCreateResource(t *testing.T) {
	t.Parallel()

	impl := initImpl(t)

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		req := &apimodels.CreateResourceRequest{
			Name:        "test-db",
			Description: "Test database",
		}

		c, rec := echotest.ContextConfig{
			JSONBody: testjsonudils.AnyToJSONBytes(t, req),
		}.ToContextRecorder(t)

		err := impl.CreateResource(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rec.Code)

		resp := testjsonudils.JSONToAny[apimodels.Resource](t, rec.Body)
		require.NotEmpty(t, resp.ID)
		require.Equal(t, req.Name, resp.Name)
		require.Equal(t, req.Description, resp.Description)
		require.False(t, resp.CreatedAt.IsZero())
	})

	t.Run("ok with external id", func(t *testing.T) {
		t.Parallel()

		externalID := "ext-123"
		req := &apimodels.CreateResourceRequest{
			Name:        "test-service",
			Description: "Test service",
			ExternalID:  &externalID,
		}

		c, rec := echotest.ContextConfig{
			JSONBody: testjsonudils.AnyToJSONBytes(t, req),
		}.ToContextRecorder(t)

		err := impl.CreateResource(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rec.Code)

		resp := testjsonudils.JSONToAny[apimodels.Resource](t, rec.Body)
		require.Equal(t, req.Name, resp.Name)
		require.NotNil(t, resp.ExternalID)
		require.Equal(t, externalID, *resp.ExternalID)
	})

	t.Run("missing name", func(t *testing.T) {
		t.Parallel()

		req := &apimodels.CreateResourceRequest{
			Description: "Test database",
		}

		c, rec := echotest.ContextConfig{
			JSONBody: testjsonudils.AnyToJSONBytes(t, req),
		}.ToContextRecorder(t)

		err := impl.CreateResource(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("missing description", func(t *testing.T) {
		t.Parallel()

		req := &apimodels.CreateResourceRequest{
			Name: "test-db",
		}

		c, rec := echotest.ContextConfig{
			JSONBody: testjsonudils.AnyToJSONBytes(t, req),
		}.ToContextRecorder(t)

		err := impl.CreateResource(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

package resourcesapi

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
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

		// A unique name guarantees a real create (CreateResource is get-or-create:
		// a name clash would return someone else's resource and its author).
		req := &apimodels.CreateResourceRequest{
			Name:        "test-db-" + uuid.NewString(),
			Description: "Test database",
		}

		c, rec := echotest.ContextConfig{
			JSONBody: testjsonudils.AnyToJSONBytes(t, req),
		}.ToContextRecorder(t)
		author := seedUser(t, c)

		err := impl.CreateResource(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rec.Code)

		resp := testjsonudils.JSONToAny[apimodels.Resource](t, rec.Body)
		require.NotEmpty(t, resp.ID)
		require.Equal(t, req.Name, resp.Name)
		require.Equal(t, req.Description, resp.Description)
		require.False(t, resp.CreatedAt.IsZero())

		// The author is captured from the token (Echo context) and resolved on the
		// response. A freshly created resource has no editor yet.
		require.NotNil(t, resp.CreatedBy)
		require.Equal(t, author.ID, resp.CreatedBy.ID)
		require.Nil(t, resp.UpdatedBy)
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
		seedUser(t, c)

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

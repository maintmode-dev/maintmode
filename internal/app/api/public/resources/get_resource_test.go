package resourcesapi

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/require"

	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/resources/models"
	testjsonudils "github.com/ruko1202/maintmode/test/utils/json"
)

func TestGetResource(t *testing.T) {
	t.Parallel()

	impl := initImpl(t)

	t.Run("ok returns the resource", func(t *testing.T) {
		t.Parallel()

		resource := makeResource(t, impl)

		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{{Name: "id", Value: resource.ID.String()}})

		err := impl.GetResource(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rec.Code)

		got := testjsonudils.JSONToAny[apimodels.Resource](t, rec.Body)
		require.Equal(t, resource.ID, got.ID)
		require.Equal(t, resource.Name, got.Name)
		require.Equal(t, "active", got.Status)
	})

	t.Run("not found on unknown id", func(t *testing.T) {
		t.Parallel()

		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{{Name: "id", Value: uuid.New().String()}})

		err := impl.GetResource(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("bad request on invalid id", func(t *testing.T) {
		t.Parallel()

		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{{Name: "id", Value: "not-a-uuid"}})

		err := impl.GetResource(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

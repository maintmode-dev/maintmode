package resourcesapi

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/require"
)

func TestArchiveResource(t *testing.T) {
	t.Parallel()

	impl := initImpl(t)

	t.Run("archives a resource and hides it from active list", func(t *testing.T) {
		t.Parallel()

		resource := makeResource(t, impl)

		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{{Name: "id", Value: resource.ID.String()}})

		err := impl.ArchiveResource(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, rec.Code)

		// archived resource no longer shows in the default (active-only) list
		require.Nil(t, findResourceByPaging(t, impl, resource.ID, url.Values{"limit": {"200"}}))
		// but appears when archived=true
		found := findResourceByPaging(t, impl, resource.ID, url.Values{
			"limit":    {"200"},
			"archived": {"true"},
		})
		require.NotNil(t, found)
		require.Equal(t, "archived", found.Status)
	})

	t.Run("idempotent on repeated archive", func(t *testing.T) {
		t.Parallel()

		resource := makeResource(t, impl)

		for range 2 {
			c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
			c.SetPathValues(echo.PathValues{{Name: "id", Value: resource.ID.String()}})

			err := impl.ArchiveResource(c)
			require.NoError(t, err)
			require.Equal(t, http.StatusNoContent, rec.Code)
		}
	})

	t.Run("unknown id is a no-op success", func(t *testing.T) {
		t.Parallel()

		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{{Name: "id", Value: uuid.New().String()}})

		err := impl.ArchiveResource(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, rec.Code)
	})

	t.Run("bad request on invalid id", func(t *testing.T) {
		t.Parallel()

		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{{Name: "id", Value: "not-a-uuid"}})

		err := impl.ArchiveResource(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

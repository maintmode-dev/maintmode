package resourcesapi

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/require"
)

func archive(t *testing.T, impl *Implementation, id uuid.UUID) {
	t.Helper()

	c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
	c.SetPathValues(echo.PathValues{{Name: "id", Value: id.String()}})

	require.NoError(t, impl.ArchiveResource(c))
	require.Equal(t, http.StatusNoContent, rec.Code)
}

func unarchive(t *testing.T, impl *Implementation, id string) *httptest.ResponseRecorder {
	t.Helper()

	c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
	c.SetPathValues(echo.PathValues{{Name: "id", Value: id}})

	require.NoError(t, impl.UnarchiveResource(c))

	return rec
}

func TestUnarchiveResource(t *testing.T) {
	t.Parallel()

	impl := initImpl(t)

	t.Run("restores resource to the active list", func(t *testing.T) {
		t.Parallel()

		resource := makeResource(t, impl)
		archive(t, impl, resource.ID)

		rec := unarchive(t, impl, resource.ID.String())
		require.Equal(t, http.StatusNoContent, rec.Code)

		found := findResourceByPaging(t, impl, resource.ID, url.Values{"limit": {"200"}})
		require.NotNil(t, found)
		require.Equal(t, "active", found.Status)
	})

	t.Run("idempotent on repeated unarchive", func(t *testing.T) {
		t.Parallel()

		resource := makeResource(t, impl)

		for range 2 {
			rec := unarchive(t, impl, resource.ID.String())
			require.Equal(t, http.StatusNoContent, rec.Code)
		}
	})

	t.Run("unknown id is a no-op success", func(t *testing.T) {
		t.Parallel()

		rec := unarchive(t, impl, uuid.New().String())
		require.Equal(t, http.StatusNoContent, rec.Code)
	})

	t.Run("bad request on invalid id", func(t *testing.T) {
		t.Parallel()

		rec := unarchive(t, impl, "not-a-uuid")
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

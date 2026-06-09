package resourcesapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/resources/models"
	"github.com/ruko1202/maintmode/internal/entity"
	testjsonudils "github.com/ruko1202/maintmode/test/utils/json"
)

func createNamedResource(t *testing.T, impl *Implementation, name string) *apimodels.Resource {
	t.Helper()

	c, rec := echotest.ContextConfig{
		JSONBody: testjsonudils.AnyToJSONBytes(t, &apimodels.CreateResourceRequest{
			Name:        name,
			Description: t.Name(),
		}),
	}.ToContextRecorder(t)
	seedUser(t, c)

	require.NoError(t, impl.CreateResource(c))
	require.Equal(t, http.StatusOK, rec.Code)

	resource := testjsonudils.JSONToAny[apimodels.Resource](t, rec.Body)

	return &resource
}

func updateResource(t *testing.T, impl *Implementation, id uuid.UUID, req *apimodels.UpdateResourceRequest) *httptest.ResponseRecorder {
	t.Helper()

	rec, _ := updateResourceAs(t, impl, id, req)
	return rec
}

// updateResourceAs runs an update with a freshly seeded editor and returns both
// the recorder and the editor that was put on the context, so tests can assert
// the resolved updated_by.
func updateResourceAs(t *testing.T, impl *Implementation, id uuid.UUID, req *apimodels.UpdateResourceRequest) (*httptest.ResponseRecorder, *entity.User) {
	t.Helper()

	c, rec := echotest.ContextConfig{
		JSONBody: testjsonudils.AnyToJSONBytes(t, req),
	}.ToContextRecorder(t)
	c.SetPathValues(echo.PathValues{{Name: "id", Value: id.String()}})
	editor := seedUser(t, c)

	err := impl.UpdateResource(c)
	require.NoError(t, err)

	return rec, editor
}

func TestUpdateResource(t *testing.T) {
	t.Parallel()

	impl := initImpl(t)

	t.Run("updates only the provided fields", func(t *testing.T) {
		t.Parallel()

		resource := makeResource(t, impl)
		newName := "Renamed" + t.Name() + uuid.NewString()

		rec := updateResource(t, impl, resource.ID, &apimodels.UpdateResourceRequest{
			Name: lo.ToPtr(newName),
		})
		require.Equal(t, http.StatusOK, rec.Code)

		got := testjsonudils.JSONToAny[apimodels.Resource](t, rec.Body)
		require.Equal(t, newName, got.Name)
		require.Equal(t, resource.Description, got.Description)
		require.Equal(t, resource.ExternalID, got.ExternalID)
	})

	t.Run("records the editor and preserves the author", func(t *testing.T) {
		t.Parallel()

		resource := makeResource(t, impl)
		require.NotNil(t, resource.CreatedBy)
		author := resource.CreatedBy.ID

		rec, editor := updateResourceAs(t, impl, resource.ID, &apimodels.UpdateResourceRequest{
			Name: lo.ToPtr("Renamed" + t.Name() + uuid.NewString()),
		})
		require.Equal(t, http.StatusOK, rec.Code)

		got := testjsonudils.JSONToAny[apimodels.Resource](t, rec.Body)
		// updated_by is the editor from the token; created_by survives the edit.
		require.NotNil(t, got.UpdatedBy)
		require.Equal(t, editor.ID, got.UpdatedBy.ID)
		require.NotNil(t, got.CreatedBy)
		require.Equal(t, author, got.CreatedBy.ID)
		require.NotEqual(t, editor.ID, author)
	})

	t.Run("clears external_id with empty string", func(t *testing.T) {
		t.Parallel()

		resource := makeResource(t, impl)

		rec := updateResource(t, impl, resource.ID, &apimodels.UpdateResourceRequest{
			ExternalID: lo.ToPtr(""),
		})
		require.Equal(t, http.StatusOK, rec.Code)

		got := testjsonudils.JSONToAny[apimodels.Resource](t, rec.Body)
		require.Empty(t, lo.FromPtr(got.ExternalID))
	})

	t.Run("nil fields leave the resource unchanged", func(t *testing.T) {
		t.Parallel()

		resource := makeResource(t, impl)

		rec := updateResource(t, impl, resource.ID, &apimodels.UpdateResourceRequest{})
		require.Equal(t, http.StatusOK, rec.Code)

		got := testjsonudils.JSONToAny[apimodels.Resource](t, rec.Body)
		require.Equal(t, resource.Name, got.Name)
		require.Equal(t, resource.Description, got.Description)
		require.Equal(t, resource.ExternalID, got.ExternalID)
	})

	t.Run("bad request on empty name", func(t *testing.T) {
		t.Parallel()

		resource := makeResource(t, impl)

		rec := updateResource(t, impl, resource.ID, &apimodels.UpdateResourceRequest{
			Name: lo.ToPtr(""),
		})
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("bad request on invalid id", func(t *testing.T) {
		t.Parallel()

		c, rec := echotest.ContextConfig{
			JSONBody: testjsonudils.AnyToJSONBytes(t, &apimodels.UpdateResourceRequest{
				Name: lo.ToPtr("whatever"),
			}),
		}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{{Name: "id", Value: "not-a-uuid"}})

		err := impl.UpdateResource(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("not found on unknown id", func(t *testing.T) {
		t.Parallel()

		rec := updateResource(t, impl, uuid.New(), &apimodels.UpdateResourceRequest{
			Name: lo.ToPtr("Name" + t.Name() + uuid.NewString()),
		})
		require.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("conflict on rename to existing name", func(t *testing.T) {
		t.Parallel()

		existing := createNamedResource(t, impl, "existing-"+t.Name()+uuid.NewString())
		other := createNamedResource(t, impl, "other-"+t.Name()+uuid.NewString())

		rec := updateResource(t, impl, other.ID, &apimodels.UpdateResourceRequest{
			Name: lo.ToPtr(existing.Name),
		})
		require.Equal(t, http.StatusConflict, rec.Code)
	})
}

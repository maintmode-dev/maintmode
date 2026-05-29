package resourcesapi

import (
	"maps"
	"net/http"
	"net/url"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5/echotest"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/resources/models"
	testjsonudils "github.com/ruko1202/maintmode/test/utils/json"
)

func TestListResources(t *testing.T) {
	t.Parallel()

	impl := initImpl(t)

	t.Run("ok lists created resource as active", func(t *testing.T) {
		t.Parallel()

		resource := makeResource(t, impl)

		// The test DB is shared, so the freshly created resource may not land on
		// the first page. Page through (newest first) until we find it.
		found := findResourceByPaging(t, impl, resource.ID, url.Values{"limit": {"200"}})
		require.NotNil(t, found)
		require.Equal(t, "active", found.Status)
	})

	t.Run("ok response carries pagination metadata", func(t *testing.T) {
		t.Parallel()

		makeResource(t, impl)

		c, rec := echotest.ContextConfig{
			QueryValues: url.Values{
				"limit":  {"5"},
				"offset": {"0"},
			},
		}.ToContextRecorder(t)

		err := impl.ListResources(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rec.Code)

		resp := testjsonudils.JSONToAny[apimodels.ListResourcesResponse](t, rec.Body)
		require.GreaterOrEqual(t, resp.Total, int64(1))
		require.Equal(t, int64(5), resp.Limit)
		require.Equal(t, int64(0), resp.Offset)
		require.LessOrEqual(t, len(resp.Resources), 5)
	})

	t.Run("ok filters by partial name", func(t *testing.T) {
		t.Parallel()

		resource := makeResource(t, impl)
		fragment := resource.Name[len(resource.Name)/2:]

		c, rec := echotest.ContextConfig{
			QueryValues: url.Values{
				"name":  {fragment},
				"limit": {"200"},
			},
		}.ToContextRecorder(t)

		err := impl.ListResources(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rec.Code)

		resp := testjsonudils.JSONToAny[apimodels.ListResourcesResponse](t, rec.Body)
		require.GreaterOrEqual(t, resp.Total, int64(1))
		_, ok := lo.Find(resp.Resources, func(item *apimodels.Resource) bool {
			return item.ID == resource.ID
		})
		require.True(t, ok)
	})

	t.Run("ok limit defaults and caps", func(t *testing.T) {
		t.Parallel()

		// limit above max falls back to the default page size
		c, rec := echotest.ContextConfig{
			QueryValues: url.Values{
				"limit": {"99999"},
			},
		}.ToContextRecorder(t)

		err := impl.ListResources(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rec.Code)

		resp := testjsonudils.JSONToAny[apimodels.ListResourcesResponse](t, rec.Body)
		require.Equal(t, int64(defaultResourcesLimit), resp.Limit)
	})

	t.Run("malformed limit falls back to default", func(t *testing.T) {
		t.Parallel()

		c, rec := echotest.ContextConfig{
			QueryValues: url.Values{
				"limit": {"abc"},
			},
		}.ToContextRecorder(t)

		err := impl.ListResources(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rec.Code)

		resp := testjsonudils.JSONToAny[apimodels.ListResourcesResponse](t, rec.Body)
		require.Equal(t, int64(defaultResourcesLimit), resp.Limit)
		require.Equal(t, int64(0), resp.Offset)
	})

	t.Run("malformed archived falls back to active-only", func(t *testing.T) {
		t.Parallel()

		c, rec := echotest.ContextConfig{
			QueryValues: url.Values{
				"archived": {"maybe"},
			},
		}.ToContextRecorder(t)

		err := impl.ListResources(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rec.Code)
	})
}

// findResourceByPaging walks the list endpoint page by page (newest first)
// until the resource with the given id is found or the list is exhausted.
// The test DB is shared between parallel tests, so a single page is not enough
// to reliably locate a freshly created resource.
func findResourceByPaging(t *testing.T, impl *Implementation, id uuid.UUID, base url.Values) *apimodels.Resource {
	t.Helper()

	offset := 0
	for {
		q := url.Values{}
		maps.Copy(q, base)
		q.Set("offset", strconv.Itoa(offset))

		c, rec := echotest.ContextConfig{QueryValues: q}.ToContextRecorder(t)
		err := impl.ListResources(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rec.Code)

		resp := testjsonudils.JSONToAny[apimodels.ListResourcesResponse](t, rec.Body)
		if found, ok := lo.Find(resp.Resources, func(item *apimodels.Resource) bool {
			return item.ID == id
		}); ok {
			return found
		}

		offset += len(resp.Resources)
		if len(resp.Resources) == 0 || int64(offset) >= resp.Total {
			return nil
		}
	}
}

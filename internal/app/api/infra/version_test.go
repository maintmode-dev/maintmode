package infra

import (
	"net/http"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/require"

	testjsonudils "github.com/ruko1202/maintmode/test/utils/json"

	"github.com/ruko1202/maintmode/internal/config"
)

func TestVersion(t *testing.T) {
	t.Parallel()
	impl := initImpl(t)

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)

		err := impl.Version(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, echo.MIMEApplicationJSON, rec.Header().Get(echo.HeaderContentType))

		resp := testjsonudils.AnyToJSON(t, config.GetAppBuildMeta())
		require.Equal(t, resp, rec.Body.String())
	})
}

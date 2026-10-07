package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/config"
)

func TestNotAllowedInProd(t *testing.T) {
	t.Parallel()

	for env, want := range map[config.Environment]int{
		config.ProdEnvironment:  http.StatusMethodNotAllowed,
		config.DevEnvironment:   http.StatusNoContent,
		config.LocalEnvironment: http.StatusNoContent,
	} {
		c, rec := echotest.ContextConfig{
			Request: httptest.NewRequest(http.MethodPost, "/", http.NoBody),
		}.ToContextRecorder(t)

		err := NotAllowedInProd(env)(func(c *echo.Context) error {
			return c.NoContent(http.StatusNoContent)
		})(c)
		require.NoError(t, err)
		require.Equal(t, want, rec.Code, env)
	}
}

package infra

import (
	"net/http"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/require"
)

func TestMainPage(t *testing.T) {
	t.Parallel()
	impl := initImpl(t)

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)

		err := impl.MainPage(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, echo.MIMETextHTMLCharsetUTF8, rec.Header().Get(echo.HeaderContentType))
		require.NotEmpty(t, rec.Body.String())
	})
}

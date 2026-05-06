package infra

import (
	"net/http"
	"testing"

	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/require"
)

func TestLiveness(t *testing.T) {
	t.Parallel()
	impl := initImpl(t)

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)

		err := impl.Liveness(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, rec.Code)
	})
}

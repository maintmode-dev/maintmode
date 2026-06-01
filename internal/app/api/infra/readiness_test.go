package infra

import (
	"net/http"
	"testing"

	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/lifecycle"
)

func TestReadiness(t *testing.T) {
	t.Parallel()
	impl := initImpl(t)

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)

		err := impl.Readiness(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, rec.Code)
	})

	t.Run("draining returns 503", func(t *testing.T) {
		t.Parallel()

		// Dedicated drainer + impl so the draining state does not leak into
		// the parallel "ok" subtest sharing the package-level db.
		drainer := lifecycle.NewDrainer()
		drainer.StartDraining()
		impl := New(db, drainer)

		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)

		err := impl.Readiness(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	})
}

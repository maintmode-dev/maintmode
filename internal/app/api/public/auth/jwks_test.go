package auth

import (
	"context"
	"net/http"
	"testing"

	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/require"

	apiauthmodels "github.com/ruko1202/maintmode/internal/app/api/public/auth/models"
	testjsonudils "github.com/ruko1202/maintmode/test/utils/json"
)

func TestJWKS(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t)
	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)

		err := impl.JWKS(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rec.Code)

		jwks := apiauthmodels.ToAPIJWKSResponse(impl.tokenSrv.JWKS(ctx))
		require.Equal(t, testjsonudils.AnyToJSON(t, jwks), rec.Body.String())
	})
}

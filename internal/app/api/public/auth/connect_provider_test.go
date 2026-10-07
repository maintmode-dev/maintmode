package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
)

// In prod, connect takes no provider credential from the client: the backend
// runs the dance, and an OAuth2 provider's token handed in by a client can
// belong to any application its owner ever authorized. The dance mode is the
// way to connect there.
func TestConnectProviderRefusesAClientCredentialInProd(t *testing.T) {
	t.Parallel()

	impl := initImpl(t)
	impl.clientCredentials = false

	req := httptest.NewRequest(http.MethodPost, "/me/providers/google/connect",
		strings.NewReader(`{"id_token":"a-token-from-anywhere"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	c := echotest.ContextConfig{
		Request:    req,
		Response:   rec,
		PathValues: echo.PathValues{{Name: "provider", Value: string(entity.AuthMethodGoogle)}},
	}.ToContext(t)
	xecho.UserToEchoCtx(c, &entity.User{ID: uuid.New(), Roles: []entity.Role{entity.RoleGuest}})

	require.NoError(t, impl.ConnectProvider(c))
	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

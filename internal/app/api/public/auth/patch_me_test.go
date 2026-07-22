package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/require"

	apiauthmodels "github.com/ruko1202/maintmode/internal/app/api/public/auth/models"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
	testjsonudils "github.com/ruko1202/maintmode/test/utils/json"
)

func TestUpdateMe(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t)

	newUser := func(t *testing.T) *entity.User {
		t.Helper()
		user, err := impl.userSrv.GetOrCreateByOAuthInfo(ctx, entity.OAuthProviderGoogle, &entity.OAuthProviderUserInfo{
			ID:    "oauth-" + uuid.NewString(),
			Email: uuid.NewString() + "@test.local",
			Name:  "Patch Me User",
		}, entity.UserCreationPolicy{AllowCreate: true})
		require.NoError(t, err)
		return user
	}

	patch := func(t *testing.T, user *entity.User, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/me", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		c, rec := echotest.ContextConfig{Request: req}.ToContextRecorder(t)
		xecho.UserToEchoCtx(c, user)
		require.NoError(t, impl.UpdateMe(c))
		return rec
	}

	t.Run("set valid timezone returns 200 and echoes it", func(t *testing.T) {
		t.Parallel()

		user := newUser(t)
		rec := patch(t, user, `{"timezone":"Asia/Nicosia"}`)
		require.Equal(t, http.StatusOK, rec.Code)

		got := testjsonudils.JSONToAny[apiauthmodels.MeResponse](t, rec.Body)
		require.NotNil(t, got.Timezone)
		require.Equal(t, "Asia/Nicosia", *got.Timezone)

		// Persisted: a subsequent GET /me sees it.
		c, meRec := echotest.ContextConfig{}.ToContextRecorder(t)
		xecho.UserToEchoCtx(c, user)
		require.NoError(t, impl.Me(c))
		me := testjsonudils.JSONToAny[apiauthmodels.MeResponse](t, meRec.Body)
		require.Equal(t, "Asia/Nicosia", *me.Timezone)
	})

	t.Run("null resets to auto-detect", func(t *testing.T) {
		t.Parallel()

		user := newUser(t)
		require.Equal(t, http.StatusOK, patch(t, user, `{"timezone":"Europe/Berlin"}`).Code)

		rec := patch(t, user, `{"timezone":null}`)
		require.Equal(t, http.StatusOK, rec.Code)
		got := testjsonudils.JSONToAny[apiauthmodels.MeResponse](t, rec.Body)
		require.Nil(t, got.Timezone)
	})

	t.Run("invalid IANA returns 400", func(t *testing.T) {
		t.Parallel()

		user := newUser(t)
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/me", strings.NewReader(`{"timezone":"Mars/Phobos"}`))
		req.Header.Set("Content-Type", "application/json")
		c, rec := echotest.ContextConfig{Request: req}.ToContextRecorder(t)
		xecho.UserToEchoCtx(c, user)

		require.NoError(t, impl.UpdateMe(c))
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("wrong-typed timezone returns 400", func(t *testing.T) {
		t.Parallel()

		user := newUser(t)
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/me", strings.NewReader(`{"timezone":123}`))
		req.Header.Set("Content-Type", "application/json")
		c, rec := echotest.ContextConfig{Request: req}.ToContextRecorder(t)
		xecho.UserToEchoCtx(c, user)

		require.NoError(t, impl.UpdateMe(c))
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("malformed JSON returns 400", func(t *testing.T) {
		t.Parallel()

		user := newUser(t)
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/me", strings.NewReader(`{"timezone":`))
		req.Header.Set("Content-Type", "application/json")
		c, rec := echotest.ContextConfig{Request: req}.ToContextRecorder(t)
		xecho.UserToEchoCtx(c, user)

		require.NoError(t, impl.UpdateMe(c))
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("missing user in context returns 401", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodPatch, "/api/v1/me", strings.NewReader(`{"timezone":"UTC"}`))
		req.Header.Set("Content-Type", "application/json")
		c, rec := echotest.ContextConfig{Request: req}.ToContextRecorder(t)

		require.NoError(t, impl.UpdateMe(c))
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

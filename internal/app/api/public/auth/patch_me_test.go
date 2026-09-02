package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5/echotest"
	"github.com/samber/lo"
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
		user, err := impl.userSrv.GetOrCreateByAuthInfo(ctx, entity.AuthMethodGoogle, &entity.OAuthProviderUserInfo{
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

	getMe := func(t *testing.T, user *entity.User) apiauthmodels.MeResponse {
		t.Helper()
		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
		xecho.UserToEchoCtx(c, user)
		require.NoError(t, impl.Me(c))
		return testjsonudils.JSONToAny[apiauthmodels.MeResponse](t, rec.Body)
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
		require.Equal(t, "Asia/Nicosia", lo.FromPtr(getMe(t, user).Timezone))
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

	t.Run("patching one field leaves the others untouched", func(t *testing.T) {
		t.Parallel()

		user := newUser(t)
		require.Equal(t, http.StatusOK, patch(t, user, `{"timezone":"Asia/Nicosia"}`).Code)

		// The regression this whole presence-tracking exists for: a patch that
		// never mentions timezone must not reset it.
		rec := patch(t, user, `{"telegram_tag":"ruslan"}`)
		require.Equal(t, http.StatusOK, rec.Code)

		got := testjsonudils.JSONToAny[apiauthmodels.MeResponse](t, rec.Body)
		require.NotNil(t, got.Timezone)
		require.Equal(t, "Asia/Nicosia", *got.Timezone)
		require.Equal(t, "ruslan", lo.FromPtr(got.TelegramTag))

		me := getMe(t, user)
		require.Equal(t, "Asia/Nicosia", lo.FromPtr(me.Timezone))
		require.Equal(t, "ruslan", lo.FromPtr(me.TelegramTag))
		require.Nil(t, me.SlackTag)
	})

	t.Run("empty object body touches nothing", func(t *testing.T) {
		t.Parallel()

		user := newUser(t)
		require.Equal(t, http.StatusOK, patch(t, user, `{"timezone":"Europe/Berlin","slack_tag":"ruslan"}`).Code)

		// "{}" is deserialized, so UnmarshalJSON runs and leaves an empty
		// presence set.
		rec := patch(t, user, `{}`)
		require.Equal(t, http.StatusOK, rec.Code)

		got := testjsonudils.JSONToAny[apiauthmodels.MeResponse](t, rec.Body)
		require.Equal(t, "Europe/Berlin", lo.FromPtr(got.Timezone))
		require.Equal(t, "ruslan", lo.FromPtr(got.SlackTag))

		me := getMe(t, user)
		require.Equal(t, "Europe/Berlin", lo.FromPtr(me.Timezone))
		require.Equal(t, "ruslan", lo.FromPtr(me.SlackTag))
	})

	t.Run("body-less request touches nothing", func(t *testing.T) {
		t.Parallel()

		user := newUser(t)
		require.Equal(t, http.StatusOK, patch(t, user, `{"timezone":"Europe/Berlin","slack_tag":"ruslan"}`).Code)

		// Separate from the "{}" case on purpose: Echo returns from BindBody
		// before deserializing when Content-Length is 0, so UnmarshalJSON never
		// runs and the presence set stays nil. Different path, same outcome.
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/me", http.NoBody)
		req.Header.Set("Content-Type", "application/json")
		require.Zero(t, req.ContentLength, "the point of this test is the empty-body path")
		c, rec := echotest.ContextConfig{Request: req}.ToContextRecorder(t)
		xecho.UserToEchoCtx(c, user)
		require.NoError(t, impl.UpdateMe(c))
		require.Equal(t, http.StatusOK, rec.Code)

		got := testjsonudils.JSONToAny[apiauthmodels.MeResponse](t, rec.Body)
		require.Equal(t, "Europe/Berlin", lo.FromPtr(got.Timezone))
		require.Equal(t, "ruslan", lo.FromPtr(got.SlackTag))

		me := getMe(t, user)
		require.Equal(t, "Europe/Berlin", lo.FromPtr(me.Timezone))
		require.Equal(t, "ruslan", lo.FromPtr(me.SlackTag))
	})

	t.Run("tags are stored exactly as entered", func(t *testing.T) {
		t.Parallel()

		user := newUser(t)
		rec := patch(t, user, `{"telegram_tag":"@ruslan","slack_tag":"ruslan"}`)
		require.Equal(t, http.StatusOK, rec.Code)

		// The leading "@" is neither added nor stripped: the stored value is
		// pasted verbatim into the outgoing notification.
		me := getMe(t, user)
		require.Equal(t, "@ruslan", lo.FromPtr(me.TelegramTag))
		require.Equal(t, "ruslan", lo.FromPtr(me.SlackTag))
	})

	t.Run("null clears a tag", func(t *testing.T) {
		t.Parallel()

		user := newUser(t)
		require.Equal(t, http.StatusOK, patch(t, user, `{"telegram_tag":"@ruslan"}`).Code)

		rec := patch(t, user, `{"telegram_tag":null}`)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Nil(t, testjsonudils.JSONToAny[apiauthmodels.MeResponse](t, rec.Body).TelegramTag)
		require.Nil(t, getMe(t, user).TelegramTag)
	})

	t.Run("reserved slack tag returns 400", func(t *testing.T) {
		t.Parallel()

		for _, tag := range []string{"@channel", "channel", "@HERE", "everyone"} {
			user := newUser(t)
			rec := patch(t, user, `{"slack_tag":"`+tag+`"}`)
			require.Equal(t, http.StatusBadRequest, rec.Code, tag)
			require.Nil(t, getMe(t, user).SlackTag, tag)
		}
	})

	t.Run("real usernames near reserved words stay legal", func(t *testing.T) {
		t.Parallel()

		user := newUser(t)
		rec := patch(t, user, `{"telegram_tag":"@admin"}`)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, "@admin", lo.FromPtr(getMe(t, user).TelegramTag))

		// A word that merely starts with a reserved one is still a username: the
		// reservation is an exact match, not a prefix.
		rec = patch(t, user, `{"telegram_tag":"@channels"}`)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, "@channels", lo.FromPtr(getMe(t, user).TelegramTag))
	})

	t.Run("invalid tag returns 400 and applies nothing", func(t *testing.T) {
		t.Parallel()

		user := newUser(t)
		require.Equal(t, http.StatusOK, patch(t, user, `{"timezone":"Europe/Berlin"}`).Code)

		// A rejection anywhere in the patch leaves the whole row alone — the
		// valid timezone in the same body must not land either.
		rec := patch(t, user, `{"timezone":"UTC","telegram_tag":"@"}`)
		require.Equal(t, http.StatusBadRequest, rec.Code)

		me := getMe(t, user)
		require.Equal(t, "Europe/Berlin", lo.FromPtr(me.Timezone))
		require.Nil(t, me.TelegramTag)
	})
}

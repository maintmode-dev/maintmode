package users

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/users/models"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
	testjsonudils "github.com/ruko1202/maintmode/test/utils/json"
)

func TestUpdateUserTags(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t)

	newUser := func(t *testing.T) *entity.User {
		t.Helper()
		user, err := impl.userSrv.GetOrCreateByOAuthInfo(ctx, entity.OAuthProviderGoogle, &entity.OAuthProviderUserInfo{
			ID:    "oauth-" + uuid.NewString(),
			Email: uuid.NewString() + "@test.local",
			Name:  "Patch Tags User",
		}, entity.UserCreationPolicy{AllowCreate: true})
		require.NoError(t, err)
		return user
	}

	// patchID sends a body to PATCH /users/{pathID} on behalf of actor. The path
	// id is passed separately from the target so the malformed-uuid case can send
	// something that is not a uuid at all.
	patchID := func(t *testing.T, actor *entity.User, pathID, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/users/"+pathID, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		c, rec := echotest.ContextConfig{
			Request:    req,
			PathValues: echo.PathValues{{Name: "id", Value: pathID}},
		}.ToContextRecorder(t)
		xecho.UserToEchoCtx(c, actor)
		require.NoError(t, impl.UpdateUserTags(c))
		return rec
	}

	patch := func(t *testing.T, actor, target *entity.User, body string) *httptest.ResponseRecorder {
		t.Helper()
		return patchID(t, actor, target.ID.String(), body)
	}

	// reload reads the target back through the listing path, i.e. the same query
	// GET /users/list serves, so a persisted value is asserted the way an admin
	// would actually see it.
	reload := func(t *testing.T, target *entity.User) *apimodels.User {
		t.Helper()
		result, err := impl.userSrv.ListUsers(ctx, &entity.ListUsersCmd{
			IDs:   []uuid.UUID{target.ID},
			Limit: 1,
		})
		require.NoError(t, err)
		require.Len(t, result.Users, 1)
		return apimodels.ToAPIUser(result.Users[0], result.ActiveAdminCount, result.ProvidersByUser[target.ID])
	}

	// "actor", not "admin": the handler never inspects roles — the admin-only
	// gate is middleware, exercised in test/api. Naming these cases "admin"
	// would promise a check this level does not perform.
	t.Run("actor sets another user's telegram tag", func(t *testing.T) {
		t.Parallel()

		admin, target := newUser(t), newUser(t)
		rec := patch(t, admin, target, `{"telegram_tag":"@ruslan"}`)
		require.Equal(t, http.StatusOK, rec.Code)

		// Stored exactly as entered: the leading "@" is neither added nor
		// stripped, because the value is pasted verbatim into notifications.
		got := testjsonudils.JSONToAny[apimodels.User](t, rec.Body)
		require.Equal(t, target.ID, got.ID)
		require.Equal(t, "@ruslan", lo.FromPtr(got.TelegramTag))
		require.Equal(t, "@ruslan", lo.FromPtr(reload(t, target).TelegramTag))
	})

	t.Run("the response carries the full listing shape", func(t *testing.T) {
		t.Parallel()

		admin, target := newUser(t), newUser(t)
		rec := patch(t, admin, target, `{"slack_tag":"ruslan"}`)
		require.Equal(t, http.StatusOK, rec.Code)

		// The fields that entity.User alone cannot supply — the linked OAuth
		// providers and is_last_admin — must be populated, so the client can drop
		// the response straight into the row it edited.
		got := testjsonudils.JSONToAny[apimodels.User](t, rec.Body)
		require.Equal(t, target.Email, got.Email)
		require.Equal(t, []string{string(entity.OAuthProviderGoogle)}, got.ConnectedProviders)
		require.Equal(t, string(entity.OAuthProviderGoogle), got.OAuthProvider)
		require.False(t, got.IsLastAdmin)
	})

	t.Run("patching one tag leaves the other untouched", func(t *testing.T) {
		t.Parallel()

		admin, target := newUser(t), newUser(t)
		require.Equal(t, http.StatusOK, patch(t, admin, target, `{"slack_tag":"ruslan"}`).Code)

		// The regression the presence set exists for: a patch that never mentions
		// slack_tag must not clear it.
		rec := patch(t, admin, target, `{"telegram_tag":"x"}`)
		require.Equal(t, http.StatusOK, rec.Code)

		got := testjsonudils.JSONToAny[apimodels.User](t, rec.Body)
		require.Equal(t, "x", lo.FromPtr(got.TelegramTag))
		require.Equal(t, "ruslan", lo.FromPtr(got.SlackTag))

		stored := reload(t, target)
		require.Equal(t, "x", lo.FromPtr(stored.TelegramTag))
		require.Equal(t, "ruslan", lo.FromPtr(stored.SlackTag))
	})

	t.Run("null clears a tag", func(t *testing.T) {
		t.Parallel()

		admin, target := newUser(t), newUser(t)
		require.Equal(t, http.StatusOK, patch(t, admin, target, `{"telegram_tag":"@ruslan"}`).Code)

		rec := patch(t, admin, target, `{"telegram_tag":null}`)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Nil(t, testjsonudils.JSONToAny[apimodels.User](t, rec.Body).TelegramTag)
		require.Nil(t, reload(t, target).TelegramTag)
	})

	t.Run("empty object body touches nothing", func(t *testing.T) {
		t.Parallel()

		admin, target := newUser(t), newUser(t)
		require.Equal(t, http.StatusOK, patch(t, admin, target, `{"telegram_tag":"@ruslan","slack_tag":"ruslan"}`).Code)

		// "{}" is deserialized, so UnmarshalJSON runs and leaves an empty presence
		// set.
		rec := patch(t, admin, target, `{}`)
		require.Equal(t, http.StatusOK, rec.Code)

		got := testjsonudils.JSONToAny[apimodels.User](t, rec.Body)
		require.Equal(t, "@ruslan", lo.FromPtr(got.TelegramTag))
		require.Equal(t, "ruslan", lo.FromPtr(got.SlackTag))

		stored := reload(t, target)
		require.Equal(t, "@ruslan", lo.FromPtr(stored.TelegramTag))
		require.Equal(t, "ruslan", lo.FromPtr(stored.SlackTag))
	})

	t.Run("body-less request touches nothing", func(t *testing.T) {
		t.Parallel()

		admin, target := newUser(t), newUser(t)
		require.Equal(t, http.StatusOK, patch(t, admin, target, `{"telegram_tag":"@ruslan","slack_tag":"ruslan"}`).Code)

		// Separate from the "{}" case on purpose: Echo returns from BindBody
		// before deserializing when Content-Length is 0, so UnmarshalJSON never
		// runs and the presence set stays nil. Different path, same outcome.
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/users/"+target.ID.String(), http.NoBody)
		req.Header.Set("Content-Type", "application/json")
		require.Zero(t, req.ContentLength, "the point of this test is the empty-body path")
		c, rec := echotest.ContextConfig{
			Request:    req,
			PathValues: echo.PathValues{{Name: "id", Value: target.ID.String()}},
		}.ToContextRecorder(t)
		xecho.UserToEchoCtx(c, admin)
		require.NoError(t, impl.UpdateUserTags(c))
		require.Equal(t, http.StatusOK, rec.Code)

		got := testjsonudils.JSONToAny[apimodels.User](t, rec.Body)
		require.Equal(t, "@ruslan", lo.FromPtr(got.TelegramTag))
		require.Equal(t, "ruslan", lo.FromPtr(got.SlackTag))

		stored := reload(t, target)
		require.Equal(t, "@ruslan", lo.FromPtr(stored.TelegramTag))
		require.Equal(t, "ruslan", lo.FromPtr(stored.SlackTag))
	})

	t.Run("reserved slack tag returns 400 and applies nothing", func(t *testing.T) {
		t.Parallel()

		for _, tag := range []string{"@channel", "channel", "@HERE", "everyone"} {
			admin, target := newUser(t), newUser(t)
			rec := patch(t, admin, target, `{"slack_tag":"`+tag+`"}`)
			require.Equal(t, http.StatusBadRequest, rec.Code, tag)
			require.Nil(t, reload(t, target).SlackTag, tag)
		}
	})

	t.Run("reserved slack values are legal telegram handles", func(t *testing.T) {
		t.Parallel()

		admin, target := newUser(t), newUser(t)
		rec := patch(t, admin, target, `{"telegram_tag":"@admin"}`)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, "@admin", lo.FromPtr(reload(t, target).TelegramTag))
	})

	t.Run("a rejected tag blocks the whole patch", func(t *testing.T) {
		t.Parallel()

		admin, target := newUser(t), newUser(t)
		rec := patch(t, admin, target, `{"telegram_tag":"@ruslan","slack_tag":"@channel"}`)
		require.Equal(t, http.StatusBadRequest, rec.Code)

		// The valid telegram_tag in the same body must not land either.
		stored := reload(t, target)
		require.Nil(t, stored.TelegramTag)
		require.Nil(t, stored.SlackTag)
	})

	t.Run("unknown user returns 404", func(t *testing.T) {
		t.Parallel()

		admin := newUser(t)
		rec := patchID(t, admin, uuid.NewString(), `{"telegram_tag":"@ruslan"}`)
		require.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("malformed uuid in the path returns 400", func(t *testing.T) {
		t.Parallel()

		admin := newUser(t)
		rec := patchID(t, admin, "not-a-uuid", `{"telegram_tag":"@ruslan"}`)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("malformed JSON returns 400", func(t *testing.T) {
		t.Parallel()

		admin, target := newUser(t), newUser(t)
		rec := patch(t, admin, target, `{"telegram_tag":`)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("missing actor in context returns 400", func(t *testing.T) {
		t.Parallel()

		target := newUser(t)
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/users/"+target.ID.String(),
			strings.NewReader(`{"telegram_tag":"@ruslan"}`))
		req.Header.Set("Content-Type", "application/json")
		c, rec := echotest.ContextConfig{
			Request:    req,
			PathValues: echo.PathValues{{Name: "id", Value: target.ID.String()}},
		}.ToContextRecorder(t)

		require.NoError(t, impl.UpdateUserTags(c))
		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Nil(t, reload(t, target).TelegramTag)
	})
}

// TestUpdateUserTagsRouteResolution pins the routing outcome that adding the
// bare PATCH /users/:id route must not disturb: the greedy param matches any
// single segment, and /users/list must keep reaching the listing handler.
//
// It asserts the outcome, not the registration order. Registration order turns
// out not to decide this: Echo v5's router is a prefix tree that prefers a
// static segment over a param no matter which was added first — verified by
// running this table with the param route registered first, where it still
// passes. The statics-before-params convention in authProtectedV1Group is
// therefore a readability rule, not the thing keeping /users/list alive, and a
// test claiming to guard the ordering would be guarding nothing.
//
// The topology below mirrors authProtectedV1Group rather than calling it,
// because the real group needs the concrete handler implementations.
func TestUpdateUserTagsRouteResolution(t *testing.T) {
	t.Parallel()

	reached := func(name string) echo.HandlerFunc {
		return func(c *echo.Context) error { return c.String(http.StatusOK, name) }
	}

	e := echo.New()
	gr := e.Group("/api/v1")

	// STATIC /users/... first.
	gr.Add(http.MethodGet, "/users/list", reached("list"))
	gr.Add(http.MethodGet, "/users/invitations", reached("invitations"))
	// PARAM routes after, bare /users/:id last of all.
	gr.Add(http.MethodPost, "/users/:id/block", reached("block"))
	gr.Add(http.MethodPatch, "/users/:id", reached("tags"))

	tests := []struct {
		name       string
		method     string
		path       string
		wantBody   string
		wantStatus int
	}{
		{
			name:       "the static list route is not shadowed by :id",
			method:     http.MethodGet,
			path:       "/api/v1/users/list",
			wantBody:   "list",
			wantStatus: http.StatusOK,
		},
		{
			name:       "the static invitations route is not shadowed by :id",
			method:     http.MethodGet,
			path:       "/api/v1/users/invitations",
			wantBody:   "invitations",
			wantStatus: http.StatusOK,
		},
		{
			name:       "the nested block route still resolves",
			method:     http.MethodPost,
			path:       "/api/v1/users/" + uuid.NewString() + "/block",
			wantBody:   "block",
			wantStatus: http.StatusOK,
		},
		{
			name:       "a uuid segment reaches the tags patch",
			method:     http.MethodPatch,
			path:       "/api/v1/users/" + uuid.NewString(),
			wantBody:   "tags",
			wantStatus: http.StatusOK,
		},
		{
			// The one case where the greedy param does bite: PATCH on a static
			// path that has no PATCH handler falls through to /users/:id and is
			// served as if "list" were a user id. It reaches the handler, which
			// rejects "list" as a malformed uuid with a 400 — a wrong-looking but
			// harmless answer, and the reason no static /users/... route may ever
			// be given a PATCH sibling without revisiting this.
			name:       "PATCH on a static path falls through to the param route",
			method:     http.MethodPatch,
			path:       "/api/v1/users/list",
			wantBody:   "tags",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(tt.method, tt.path, http.NoBody)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			require.Equal(t, tt.wantStatus, rec.Code)
			require.Equal(t, tt.wantBody, rec.Body.String())
		})
	}
}

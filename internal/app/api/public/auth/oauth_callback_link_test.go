package auth

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

// A link-mode dance reports itself with ?linked and mints no sign-in code.
//
// The branch had no test at all: every other callback test drives the sign-in
// outcome, so `linked` was produced by one line and pinned by none.
//
// The account is created holding a GITHUB identity and the dance links GOOGLE,
// because a dance that links the provider the account already signed in with is
// answered link_conflict -- correctly, and that is a different branch.
func TestCallbackLinkModeRedirectsWithLinkedAndNoCode(t *testing.T) {
	ctx := t.Context()
	impl := initDanceImpl(t)

	user, err := impl.userSrv.GetOrCreateByAuthInfo(ctx, entity.AuthMethodGithub,
		&entity.OAuthProviderUserInfo{
			ID:    "gh-" + uuid.NewString(),
			Email: uuid.NewString() + "@test.local",
			Name:  "Link Test User",
		}, entity.UserCreationPolicy{AllowCreate: true})
	require.NoError(t, err)

	ticket, err := impl.authSrv.MintLinkTicket(ctx, user.ID, string(entity.AuthMethodGoogle))
	require.NoError(t, err)

	// /start with the ticket, so the link cookie is parked the way a browser
	// receives it rather than hand-built here.
	startRec := httptest.NewRecorder()
	startCtx := echotest.ContextConfig{
		Request: httptest.NewRequest(http.MethodGet,
			"/login/oauth/"+string(entity.AuthMethodGoogle)+"/start?"+
				url.Values{paramLink: {ticket}}.Encode(), http.NoBody),
		Response:   startRec,
		PathValues: echo.PathValues{{Name: "provider", Value: string(entity.AuthMethodGoogle)}},
	}.ToContext(t)
	require.NoError(t, impl.StartOAuthDance(startCtx))

	target, err := url.Parse(startRec.Header().Get(echo.HeaderLocation))
	require.NoError(t, err)

	cookies := danceCookies(t, startRec)
	require.Contains(t, cookies, oauthLinkCookie, "/start must park the ticket in a cookie")

	rec := driveCallback(t, impl, string(entity.AuthMethodGoogle),
		url.Values{"code": {"provider-auth-code"}, "state": {target.Query().Get("state")}},
		[]*http.Cookie{
			{Name: oauthStateCookie, Value: cookies[oauthStateCookie].Value},
			{Name: oauthVerifierCookie, Value: cookies[oauthVerifierCookie].Value},
			{Name: oauthLinkCookie, Value: cookies[oauthLinkCookie].Value},
		})

	_, q := redirectResult(t, rec)

	assert.Equal(t, "1", q.Get(paramLinked), "a completed link reports itself")
	assert.Empty(t, q.Get(paramCode),
		"a link mints no session, so no sign-in code may ride the redirect")
	assert.Empty(t, q.Get(paramError))
}

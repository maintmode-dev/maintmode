package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
)

// linkTestUser creates an account holding a GITHUB identity, so a dance that
// links GOOGLE is a real link rather than a link_conflict -- correctly
// answered, and a different branch.
func linkTestUser(t *testing.T, impl *Implementation) *entity.User {
	t.Helper()

	user, err := impl.userSrv.GetOrCreateByAuthInfo(t.Context(), entity.AuthMethodGithub,
		&entity.OAuthProviderUserInfo{
			ID:    "gh-" + uuid.NewString(),
			Email: uuid.NewString() + "@test.local",
			Name:  "Link Test User",
		}, entity.UserCreationPolicy{AllowCreate: true})
	require.NoError(t, err)

	return user
}

// danceToLinkCode mints a ticket for owner, runs /start and the callback the
// way a browser does, and returns the redirect's query.
func danceToLinkCode(t *testing.T, impl *Implementation, owner *entity.User) url.Values {
	t.Helper()

	ticket, err := impl.authSrv.MintLinkTicket(t.Context(), owner.ID, string(entity.AuthMethodGoogle))
	require.NoError(t, err)

	startRec := httptest.NewRecorder()
	startCtx := echotest.ContextConfig{
		Request: httptest.NewRequest(http.MethodGet,
			"/login/oauth/"+string(entity.AuthMethodGoogle)+"/start?"+
				url.Values{paramLink: {ticket}, paramBinding: {testBinding}}.Encode(), http.NoBody),
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
			{Name: oauthBindingCookie, Value: cookies[oauthBindingCookie].Value},
		})

	_, q := redirectResult(t, rec)

	return q
}

// completeLinkAs drives POST /me/providers/link/complete from caller's session.
func completeLinkAs(t *testing.T, impl *Implementation, caller *entity.User, linkCode, proof string) *httptest.ResponseRecorder {
	t.Helper()

	body := `{"link_code":"` + linkCode + `","binding_proof":"` + proof + `"}`
	request := httptest.NewRequest(http.MethodPost, "/me/providers/link/complete", strings.NewReader(body))
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	rec := httptest.NewRecorder()
	c := echotest.ContextConfig{Request: request, Response: rec}.ToContext(t)
	xecho.UserToEchoCtx(c, caller)

	_ = impl.CompleteLink(c)

	return rec
}

func connectedProviders(t *testing.T, impl *Implementation, user *entity.User) []entity.AuthMethod {
	t.Helper()

	providers, err := impl.userSrv.ListConnectedProviders(t.Context(), user.ID)
	require.NoError(t, err)

	return providers
}

func linkErrorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()

	var resp httperrors.ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	return resp.Code
}

// A link-mode callback attaches nothing and mints no sign-in code: it hands back
// a link code, and only the owner's session redeeming it attaches the identity.
func TestLinkDanceCompletesFromTheOwnersSession(t *testing.T) {
	impl := initDanceImpl(t)
	owner := linkTestUser(t, impl)

	q := danceToLinkCode(t, impl, owner)
	require.NotEmpty(t, q.Get(paramLinkCode), "a link-mode callback hands back a link code")
	assert.Empty(t, q.Get(paramCode), "a link mints no session, so no sign-in code may ride the redirect")
	assert.Empty(t, q.Get(paramError))
	assert.NotContains(t, connectedProviders(t, impl, owner), entity.AuthMethodGoogle,
		"the callback alone must not attach the identity")

	rec := completeLinkAs(t, impl, owner, q.Get(paramLinkCode), testBindingNonce)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	assert.Contains(t, connectedProviders(t, impl, owner), entity.AuthMethodGoogle)
}

// The planted-link attack: someone mints a ticket for their OWN account and
// sends the /start URL to a colleague. The colleague's browser completes the
// provider round trip, but the colleague's session is not the ticket's owner,
// so the colleague's provider identity is never attached to the sender.
func TestLinkCodeIsRefusedToAnotherAccount(t *testing.T) {
	impl := initDanceImpl(t)
	sender := linkTestUser(t, impl)
	colleague := linkTestUser(t, impl)

	q := danceToLinkCode(t, impl, sender)

	rec := completeLinkAs(t, impl, colleague, q.Get(paramLinkCode), testBindingNonce)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Equal(t, string(httperrors.ErrLinkInvalid), linkErrorCode(t, rec))
	assert.NotContains(t, connectedProviders(t, impl, sender), entity.AuthMethodGoogle)
	assert.NotContains(t, connectedProviders(t, impl, colleague), entity.AuthMethodGoogle)

	// Spent by the refused attempt: the owner cannot pick it up afterwards
	// either, so a planted code is worth one guess.
	again := completeLinkAs(t, impl, sender, q.Get(paramLinkCode), testBindingNonce)
	assert.Equal(t, http.StatusBadRequest, again.Code)
}

// Every reason a link code cannot be redeemed answers the same 400 link_invalid
// -- never a 401, which would tell the BFF the caller's session is dead.
func TestLinkCodeFailuresAreOneAnswer(t *testing.T) {
	impl := initDanceImpl(t)
	owner := linkTestUser(t, impl)

	cases := map[string]func() *httptest.ResponseRecorder{
		"wrong proof": func() *httptest.ResponseRecorder {
			return completeLinkAs(t, impl, owner, danceToLinkCode(t, impl, owner).Get(paramLinkCode), "another-browser")
		},
		"no proof": func() *httptest.ResponseRecorder {
			return completeLinkAs(t, impl, owner, danceToLinkCode(t, impl, owner).Get(paramLinkCode), "")
		},
		"unknown code": func() *httptest.ResponseRecorder {
			return completeLinkAs(t, impl, owner, "never-issued", testBindingNonce)
		},
		"empty code": func() *httptest.ResponseRecorder {
			return completeLinkAs(t, impl, owner, "", testBindingNonce)
		},
	}

	seen := map[string]struct{}{}
	for name, run := range cases {
		t.Run(name, func(t *testing.T) {
			rec := run()
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			seen[rec.Body.String()] = struct{}{}
		})
	}

	assert.Len(t, seen, 1, "every failure must answer with one identical body")
	assert.NotContains(t, connectedProviders(t, impl, owner), entity.AuthMethodGoogle)
}

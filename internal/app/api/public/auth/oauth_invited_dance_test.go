package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/app/bootstrap"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/storages/oauthdance"
	"github.com/ruko1202/maintmode/internal/storages/userinvitations"
	"github.com/ruko1202/maintmode/internal/utils/xhash"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

// invitedRun is a danceRun plus the invitation handle /start planted.
type invitedRun struct {
	danceRun
	invitationHandle string
}

// startInvited drives /start?invitation=<token> and collects every cookie,
// including the invitation handle.
func startInvited(t *testing.T, impl *Implementation, invitationToken string) invitedRun {
	t.Helper()

	rec := httptest.NewRecorder()
	c := echotest.ContextConfig{
		Request: httptest.NewRequest(http.MethodGet,
			"/login/oauth/google/start?invitation="+url.QueryEscape(invitationToken), http.NoBody),
		Response:   rec,
		PathValues: echo.PathValues{{Name: "provider", Value: "google"}},
	}.ToContext(t)

	require.NoError(t, impl.StartOAuthDance(c))

	target, err := url.Parse(rec.Header().Get(echo.HeaderLocation))
	require.NoError(t, err)

	cookies := danceCookies(t, rec)

	run := invitedRun{danceRun: danceRun{
		state:     target.Query().Get("state"),
		signature: cookies[oauthStateCookie].Value,
		verifier:  cookies[oauthVerifierCookie].Value,
	}}
	if handle, ok := cookies[oauthInvitationCookie]; ok {
		run.invitationHandle = handle.Value
	}

	return run
}

// completeInvited drives the callback carrying the invitation handle.
func completeInvited(t *testing.T, impl *Implementation, run invitedRun) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		"/login/oauth/google/callback?"+url.Values{
			paramCode:  {"authorization-code"},
			paramState: {run.state},
		}.Encode(), http.NoBody)
	req.AddCookie(&http.Cookie{Name: oauthStateCookie, Value: run.signature})
	req.AddCookie(&http.Cookie{Name: oauthVerifierCookie, Value: run.verifier})
	if run.invitationHandle != "" {
		req.AddCookie(&http.Cookie{Name: oauthInvitationCookie, Value: run.invitationHandle})
	}

	c := echotest.ContextConfig{
		Request:    req,
		Response:   rec,
		PathValues: echo.PathValues{{Name: "provider", Value: "google"}},
	}.ToContext(t)

	require.NoError(t, impl.OAuthDanceCallback(c))

	return rec
}

// redirectError pulls the ?error= code out of the redirect the callback issued.
func redirectError(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()

	target, err := url.Parse(rec.Header().Get(echo.HeaderLocation))
	require.NoError(t, err)

	return target.Query().Get(paramError)
}

// inviteFor writes a real pending invitation straight through the store and
// returns its raw token.
//
// It goes through the store rather than invitation.Create because the raw token
// only ever exists inside the emailed link -- Create hashes it before returning
// -- and reconstructing it from a captured email would test the mail template
// rather than the dance. The row is identical either way: what the dance reads
// is the token hash, the status and the roles.
func inviteFor(t *testing.T, email string, roles ...entity.Role) string {
	t.Helper()

	if len(roles) == 0 {
		roles = []entity.Role{entity.RoleEditor}
	}

	raw := xuuid.NewString()

	// invited_by_id carries a foreign key, so the row needs a real inviter.
	stores, err := bootstrap.NewStores(db, valkey)
	require.NoError(t, err)
	services, err := bootstrap.NewServices(t.Context(), cfg, stores)
	require.NoError(t, err)

	inviter, err := services.User.GetOrCreateByAuthInfo(context.Background(), entity.AuthMethodGoogle,
		&entity.OAuthProviderUserInfo{
			ID: xuuid.NewString(), Email: xuuid.NewString() + "@inviter-dance.com", Name: "Inviter",
		}, entity.UserCreationPolicy{AllowCreate: true})
	require.NoError(t, err)

	_, err = userinvitations.NewStore(db).Create(context.Background(), &entity.Invitation{
		Email:       email,
		Roles:       roles,
		TokenHash:   xhash.HashSha256([]byte(raw)),
		Status:      entity.InvitationStatusPending,
		ExpiresAt:   xtime.UTCNow().Add(time.Hour),
		SentAt:      xtime.UTCNow(),
		InvitedByID: inviter.ID,
	})
	require.NoError(t, err)

	return raw
}

// TestInvitedDanceCreatesTheUserWithItsRoles drives the whole invited path and
// asserts what the branch exists to deliver: the account is created, carries the
// invitation's roles, and the invitation is spent.
//
// Its paired negative control below asserts that an uninvited dance inherits no
// roles. NOTE what that pair does NOT cover: hard-coding AllowCreate: true is
// invisible here, because the local stand runs allow_open_signup: true and an
// uninvited dance legitimately creates a user either way. Proving the
// AllowCreate gate needs a stand with open signup off.
func TestInvitedDanceCreatesTheUserWithItsRoles(t *testing.T) {
	t.Parallel()

	email := xuuid.NewString() + "@invited-dance.com"
	token := inviteFor(t, email, entity.RoleEditor)

	impl := initDanceImplWith(t, testRedirectURI, newFakeGateway(t, email, nil))

	run := startInvited(t, impl, token)
	require.NotEmpty(t, run.invitationHandle, "/start must plant the invitation cookie")

	rec := completeInvited(t, impl, run)
	assert.Empty(t, redirectError(t, rec), "an invited dance must complete")

	target, err := url.Parse(rec.Header().Get(echo.HeaderLocation))
	require.NoError(t, err)
	assert.NotEmpty(t, target.Query().Get(paramCode), "the frontend needs a one-time code")

	// The point of the whole branch, and the assertions that make this test
	// worth having: the account exists and carries the invitation's roles, and
	// the invitation is spent. Without them the test passes even with phase 2
	// deleted entirely -- a dance that signs people in and grants nothing.
	stores, err := bootstrap.NewStores(db, valkey)
	require.NoError(t, err)
	services, err := bootstrap.NewServices(t.Context(), cfg, stores)
	require.NoError(t, err)

	user, err := services.User.GetByEmail(context.Background(), email)
	require.NoError(t, err, "the invited person must exist afterwards")
	assert.Contains(t, user.Roles, entity.RoleEditor,
		"the invitation's roles must reach the account")

	invitations, err := services.Invitation.List(context.Background(), &entity.ListInvitationsCmd{})
	require.NoError(t, err)

	var found bool
	for _, item := range invitations {
		if item.Invitation.Email == email {
			found = true
			assert.Equal(t, entity.InvitationStatusAccepted, item.Invitation.Status,
				"the invitation must be spent, not left reusable")
		}
	}
	assert.True(t, found, "the invitation must still be listed")
}

// TestUninvitedDanceDoesNotGetInvitationRoles is the negative control for the
// test above: without an invitation, no invitation roles are granted.
//
// It asserts ROLES rather than a refusal because the local stand runs
// allow_open_signup: true, so an uninvited dance legitimately succeeds there and
// a refusal assertion would be testing the stand's config, not this code. The
// mutation it kills is the one that matters: an implementation that hard-codes
// AllowCreate: true and grants the roles of whatever invitation it can find
// would fail here, because this user gets only the defaults.
func TestUninvitedDanceDoesNotGetInvitationRoles(t *testing.T) {
	t.Parallel()

	email := xuuid.NewString() + "@uninvited-dance.com"

	// A live invitation exists for a DIFFERENT address. An uninvited dance must
	// not pick it up.
	_ = inviteFor(t, xuuid.NewString()+"@somebody-else.com", entity.RoleAdmin)

	impl := initDanceImplWith(t, testRedirectURI, newFakeGateway(t, email, nil))

	// runStart's own require.Len(cookies, 2) is what proves no handle was
	// planted; asserting run.invitationHandle here would compare a struct field
	// against the zero value it was built with and pass against anything.
	run := invitedRun{danceRun: runStart(t, impl)}
	require.Empty(t, redirectError(t, completeInvited(t, impl, run)))

	stores, err := bootstrap.NewStores(db, valkey)
	require.NoError(t, err)
	services, err := bootstrap.NewServices(t.Context(), cfg, stores)
	require.NoError(t, err)

	user, err := services.User.GetByEmail(context.Background(), email)
	require.NoError(t, err)
	assert.NotContains(t, user.Roles, entity.RoleAdmin,
		"an uninvited sign-in must not inherit an unrelated invitation's roles")
}

// TestInvitedDanceRefusesAMismatchedAccount is criterion 3: the anti-takeover
// guard, and the redirect code the person can act on.
func TestInvitedDanceRefusesAMismatchedAccount(t *testing.T) {
	t.Parallel()

	invited := xuuid.NewString() + "@invited-mismatch.com"
	token := inviteFor(t, invited)

	// The provider reports a DIFFERENT address than the one invited.
	other := xuuid.NewString() + "@someone-else.com"
	impl := initDanceImplWith(t, testRedirectURI, newFakeGateway(t, other, nil))

	rec := completeInvited(t, impl, startInvited(t, impl, token))

	assert.Equal(t, errCodeEmailMismatch, redirectError(t, rec),
		"the one failure a legitimate person can fix must say so")
}

// TestInvitedDanceRefusesAHandleThatNamesNothing is criterion 4, in its
// mutation-sensitive form. A dance with NO handle already ends in access_denied
// on unmodified main, so only a handle that resolves to nothing proves the
// resolution path refuses.
func TestInvitedDanceRefusesAHandleThatNamesNothing(t *testing.T) {
	t.Parallel()

	email := xuuid.NewString() + "@dead-invite.com"
	impl := initDanceImplWith(t, testRedirectURI, newFakeGateway(t, email, nil))

	// A token that names no invitation: /start must still hand out a handle,
	// which is what keeps it from being an oracle.
	run := startInvited(t, impl, xuuid.NewString())
	require.NotEmpty(t, run.invitationHandle,
		"/start must mint a handle even for a token that names nothing")

	rec := completeInvited(t, impl, run)
	assert.Equal(t, errCodeAccessDenied, redirectError(t, rec))
}

// TestInvitedDanceIsSingleUse: the invitation must not onboard two accounts.
// The handle is consumed on the first callback, so a replay finds nothing.
func TestInvitedDanceIsSingleUse(t *testing.T) {
	t.Parallel()

	email := xuuid.NewString() + "@single-use-dance.com"
	token := inviteFor(t, email)
	impl := initDanceImplWith(t, testRedirectURI, newFakeGateway(t, email, nil))

	run := startInvited(t, impl, token)
	require.Empty(t, redirectError(t, completeInvited(t, impl, run)))

	// The handle itself must be gone from the store. Asserting only the second
	// callback's redirect code proves less than it looks: the invitation is
	// accepted by then, so the status check refuses the replay even if the
	// handle were still live and re-redeemable. This pins the consuming read.
	consumed, err := oauthdance.NewStore(valkey, cfg.Auth.DanceStateTTL()).
		ConsumeInvitationHandle(context.Background(), run.invitationHandle)
	require.NoError(t, err)
	assert.Nil(t, consumed, "the first callback must have consumed the handle")

	// And the replay is refused.
	rec := completeInvited(t, impl, run)
	assert.Equal(t, errCodeAccessDenied, redirectError(t, rec),
		"a spent handle must not complete a second dance")
}

// TestStartDoesNotLeakTheInvitationToken is criterion 6's outbound half: the
// raw token must not reach the provider. The log half is covered by the
// sanitizer test in internal/server/middlewares.
func TestStartDoesNotLeakTheInvitationToken(t *testing.T) {
	t.Parallel()

	email := xuuid.NewString() + "@no-leak.com"
	token := inviteFor(t, email)
	impl := initDanceImplWith(t, testRedirectURI, newFakeGateway(t, email, nil))

	rec := httptest.NewRecorder()
	c := echotest.ContextConfig{
		Request: httptest.NewRequest(http.MethodGet,
			"/login/oauth/google/start?invitation="+url.QueryEscape(token), http.NoBody),
		Response:   rec,
		PathValues: echo.PathValues{{Name: "provider", Value: "google"}},
	}.ToContext(t)
	require.NoError(t, impl.StartOAuthDance(c))

	location := rec.Header().Get(echo.HeaderLocation)
	assert.NotContains(t, location, token,
		"the invitation token must never reach the provider's authorization URL")

	for name, cookie := range danceCookies(t, rec) {
		assert.NotContains(t, cookie.Value, token,
			"cookie %s must carry a handle, never the token itself", name)
	}
}

// TestPlainStartClearsAStaleInvitationCookie is the carryover guard.
//
// The state and verifier cookies are overwritten on every /start, so an
// abandoned dance cannot bleed into the next one. The invitation cookie must
// behave the same way or a handle outlives the dance it was minted for: Alice
// opens her invitation link, abandons it at the consent screen, then signs in
// normally a minute later -- and the callback applies an invitation to a dance
// that never asked for one. With a different account that is a dead-end
// email_mismatch on what should have been an ordinary login.
//
// It also matters with an attacker: cookies are not origin-isolated, so anyone
// who can set one for this domain could otherwise attach a handle of their
// choosing to somebody else's sign-in.
func TestPlainStartClearsAStaleInvitationCookie(t *testing.T) {
	t.Parallel()

	email := xuuid.NewString() + "@stale-cookie.com"
	token := inviteFor(t, email)
	impl := initDanceImplWith(t, testRedirectURI, newFakeGateway(t, email, nil))

	// An invited /start plants a handle...
	invited := startInvited(t, impl, token)
	require.NotEmpty(t, invited.invitationHandle)

	// ...and a plain /start right afterwards must actively cancel it.
	//
	// Read the RAW Set-Cookie headers, not danceCookies: that helper drops
	// expired cookies, so an implementation that says nothing about the
	// invitation cookie and one that clears it look identical through it. The
	// difference is the whole point -- saying nothing leaves the browser's copy
	// alive.
	rec := startRequest(t, impl)

	var sawInvitationCookie bool
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name != oauthInvitationCookie {
			continue
		}
		sawInvitationCookie = true
		assert.True(t, cookie.MaxAge < 0 || cookie.Value == "",
			"a plain /start must cancel a previous dance's invitation handle, got %q", cookie.Value)
	}

	assert.True(t, sawInvitationCookie,
		"a plain /start must say something about the invitation cookie; silence leaves a "+
			"stale handle live in the browser for the rest of its TTL")
}

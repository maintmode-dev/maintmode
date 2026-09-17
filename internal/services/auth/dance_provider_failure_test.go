package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	mock_auth "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/services/auth"
	"github.com/ruko1202/maintmode/internal/services/authmethod"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

// danceFailureReason runs one credential-verification failure through
// issueDanceCode and returns the reason it filed.
//
// Driven at the service level rather than through the handler because the
// distinction under test is invisible from outside: every one of these answers
// the browser with the same provider_error redirect. The audit row is the only
// place they differ, which is the whole point of the branch.
func danceFailureReason(t *testing.T, verifyErr error) entity.AuditFailureReason {
	t.Helper()

	srv, mocks := initServiceForMethod(t, entity.AuthMethodGithub)
	publisher := newRecordingAuditPublisher()
	srv.auditPublisher = publisher

	mocks.authMethod.EXPECT().
		Authenticate(gomock.Any(), "credential").
		Return(nil, verifyErr)

	// No link ticket and no invitation handle: this exercises the ordinary
	// sign-in path's credential-verification failure.
	_, err := srv.issueDanceCode(t.Context(), entity.AuthMethodGithub, "credential", "", "",
		&entity.AuditMetadata{IP: "203.0.113.9"})
	require.Error(t, err)

	// The browser-visible answer is unchanged whatever the reason: the wrap is
	// what routes every one of these to provider_error, and only the trail
	// separates them.
	assert.ErrorIs(t, err, apperr.ErrOAuthExchangeFailed)

	actions := publisher.actions()
	require.Len(t, actions, 1)

	failed, ok := actions[0].(audit.LoginFailed)
	require.True(t, ok, "a dance refusal is a login failure")
	require.NotNil(t, failed.Meta)

	return failed.Meta.FailureReason
}

// TestIssueDanceCodeAuditsAGithubAccountProblemAsProviderRejected is the
// distinction the reason exists for.
//
// "The provider is unavailable" and "this account has no address we can use"
// are different incidents with different runbooks. An operator seeing a run of
// provider-unavailable rows reasonably reaches for a rotated client_secret or an
// upstream outage; for these two sentinels nothing is broken at all, and the
// person fixes it on GitHub by verifying their primary address. Filing them as
// an outage would send that operator chasing a fault that does not exist.
func TestIssueDanceCodeAuditsAGithubAccountProblemAsProviderRejected(t *testing.T) {
	t.Parallel()

	for _, sentinel := range []error{
		apperr.ErrGithubEmailUnusable,
		apperr.ErrGithubIdentityUnusable,
	} {
		t.Run(sentinel.Error(), func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, entity.AuditFailureProviderRejected, danceFailureReason(t, sentinel))
		})
	}
}

// TestIssueDanceCodeStillAuditsAnUpstreamFailureAsUnavailable is the other half.
//
// The branch could be written to file everything as rejected, and the test above
// would still pass. A genuine upstream failure -- a refused token, an
// unreachable API -- must keep reading as an outage.
func TestIssueDanceCodeStillAuditsAnUpstreamFailureAsUnavailable(t *testing.T) {
	t.Parallel()

	for _, sentinel := range []error{
		apperr.ErrAuthUnavailable,
		apperr.ErrInvalidAccessToken,
	} {
		t.Run(sentinel.Error(), func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, entity.AuditFailureProviderUnavailable, danceFailureReason(t, sentinel))
		})
	}
}

// TestStartDanceRefusesOnALinkTicketStoreFailure is criterion 23, and it pins
// the rule a green suite would otherwise not notice.
//
// A store that cannot answer is NOT a miss. Reading it as "no ticket" would let
// the dance fall through to sign-in mode, so a Valkey outage would sign the
// person in as whichever provider account the browser happened to be holding --
// when what they asked for was to attach it to the account they already have.
//
// Driven at the service level because the distinction is invisible from outside:
// a spent ticket and an outage are told apart by the SENTINEL, not by the
// status. An earlier version of this comment claimed both answered 400 -- they
// both answered 500, and asserting that false property at the handler level was
// skipped on the strength of it. The handler tests now assert the status by
// value; this one keeps pinning the half only the service can see.
func TestStartDanceRefusesOnALinkTicketStoreFailure(t *testing.T) {
	t.Parallel()

	srv, _ := initServiceForMethod(t, entity.AuthMethodGithub)
	srv = srv.WithAuthMethods(danceableMethods{
		inner:  srv.authMethods,
		method: entity.AuthMethodGithub,
	})

	codes := mock_auth.NewMockDanceCodeStore(gomock.NewController(t))
	codes.EXPECT().
		PeekLinkTicket(gomock.Any(), "some-ticket").
		Return(nil, errors.New("valkey is unreachable"))

	srv = srv.WithDance(config.Auth{}, codes)

	_, err := srv.StartDance(t.Context(), string(entity.AuthMethodGithub), "", "some-ticket")

	require.Error(t, err, "a store failure must refuse, never fall through to sign-in")
	assert.NotErrorIs(t, err, apperr.ErrLinkTicketUnusable,
		"an outage is not the same answer as a spent ticket")
}

// seedUser creates a user the link tests can attach an identity to.
//
// Through a DIFFERENT provider than the one under test, so the account exists
// without already holding a github identity -- which is the state a real person
// is in when they ask to link one.
func seedUser(t *testing.T, svc *Service) *entity.User {
	t.Helper()

	user, err := svc.usersSrv.GetOrCreateByAuthInfo(t.Context(), entity.AuthMethodGoogle,
		&entity.OAuthProviderUserInfo{
			ID:    xuuid.NewString(),
			Email: xuuid.NewString() + "@link-test.example",
			Name:  "Linker",
		}, entity.UserCreationPolicy{AllowCreate: true})
	require.NoError(t, err)

	return user
}

// loginFailureRows returns the login-failure actions published, so a test can
// assert that a link refusal did NOT become one.
func loginFailureRows(published []audit.Action) []audit.LoginFailed {
	rows := make([]audit.LoginFailed, 0, len(published))
	for _, action := range published {
		if failed, ok := action.(audit.LoginFailed); ok {
			rows = append(rows, failed)
		}
	}

	return rows
}

// linkAuditRows returns the audit actions one link attempt published.
func linkAuditRows(t *testing.T, published []audit.Action) []audit.ProviderLinked {
	t.Helper()

	rows := make([]audit.ProviderLinked, 0, len(published))
	for _, action := range published {
		if linked, ok := action.(audit.ProviderLinked); ok {
			rows = append(rows, linked)
		}
	}

	return rows
}

// TestCompleteLinkAuditsByAction is criterion 21, and it asserts the ACTION
// rather than "a row appeared".
//
// "A row under the auth filter" is satisfied by login.failed too, which is
// exactly the mistake the separate action exists to prevent: a link filed under
// login.* corrupts the facet where a run of failures reads as someone guessing
// credentials. Asserting the concrete type is what catches that.
func TestCompleteLinkAuditsByAction(t *testing.T) {
	t.Parallel()

	srv, _ := initServiceForMethod(t, entity.AuthMethodGithub)
	publisher := newRecordingAuditPublisher()
	srv.auditPublisher = publisher

	codes := mock_auth.NewMockDanceCodeStore(gomock.NewController(t))
	// Redeems to nothing: the refusal path, which is the one a 302 leaves no
	// other trace of.
	codes.EXPECT().ConsumeLinkTicket(gomock.Any(), "spent").Return(nil, nil)

	srv = srv.WithDance(config.Auth{}, codes)

	_, err := srv.completeLink(t.Context(), entity.AuthMethodGithub, "spent",
		&entity.OAuthIDTokenClaims{Subject: "1", Email: "a@example.com"},
		&entity.AuditMetadata{IP: "203.0.113.9"})
	require.ErrorIs(t, err, apperr.ErrLinkTicketUnusable)

	rows := linkAuditRows(t, publisher.actions())
	require.Len(t, rows, 1, "a refused link must publish exactly one provider.linked row")

	// The concrete TYPE is the assertion: a login failure carrying a link-shaped
	// reason would satisfy "a row appeared" and "a row under the auth filter"
	// alike, and it is precisely what the separate action exists to prevent.
	require.NotNil(t, rows[0].Meta)
	assert.Equal(t, entity.AuditFailureLinkUnusable, rows[0].Meta.FailureReason)
}

// TestProviderLinkedIsRenderableAndFilterable pins the four edit sites that fail
// SILENTLY.
//
// The action string and the IsValid arm are loud -- nothing compiles or passes
// without them. The two category maps are not: without auditActionCategories the
// renderer is never reached and the row never renders at all, and without
// auditCategoriesAction the row renders and is invisible under the auth filter
// an operator actually uses.
func TestProviderLinkedIsRenderableAndFilterable(t *testing.T) {
	t.Parallel()

	assert.True(t, entity.AuditActionProviderLinked.IsValid())

	category, ok := entity.AuditActionCategory(entity.AuditActionProviderLinked)
	require.True(t, ok, "without a category the renderer is never reached and the row never renders")
	assert.Equal(t, entity.AuditCategoryAuth, category)

	assert.Contains(t, entity.AuditCategoryAction(entity.AuditCategoryAuth),
		entity.AuditActionProviderLinked,
		"without the reverse map the row renders but is invisible under the auth filter")
}

// TestCompleteLinkAuditsAConflictAsProviderLinked closes criterion 21's other
// half: a REFUSED-BY-CONFLICT link.
//
// The conflict path is a different branch of refuseLink than the unusable-ticket
// one above, and it was previously unreachable from any assertion: the
// handler-level conflict tests assert the redirect code only, and their harness
// records no audit at all. Three mutations shipped green because of that --
// deleting the publish entirely, swapping the reason, and (the one criterion 21
// exists to catch) publishing through publishLoginFailure instead, which files a
// link under the login facet where a run of failures reads as credential
// guessing.
//
// Driven through a REAL conflict rather than a mocked one: usersSrv is a
// concrete service, so the identity has to genuinely exist first.
func TestCompleteLinkAuditsAConflictAsProviderLinked(t *testing.T) {
	t.Parallel()

	srv, _ := initServiceForMethod(t, entity.AuthMethodGithub)

	owner := seedUser(t, srv)
	subject := xuuid.NewString()
	claims := &entity.OAuthIDTokenClaims{
		Subject:       subject,
		Email:         xuuid.NewString() + "@example.com",
		Name:          "Linker",
		EmailVerified: true,
	}

	// First link succeeds and creates the row the second one collides with.
	require.NoError(t, srv.usersSrv.LinkIdentity(t.Context(), owner.ID, entity.AuthMethodGithub, claims))

	publisher := newRecordingAuditPublisher()
	srv.auditPublisher = publisher

	codes := mock_auth.NewMockDanceCodeStore(gomock.NewController(t))
	codes.EXPECT().ConsumeLinkTicket(gomock.Any(), "ticket").Return(&entity.LinkIntent{
		UserID:   owner.ID,
		Provider: entity.AuthMethodGithub,
	}, nil)

	srv = srv.WithDance(config.Auth{}, codes)

	_, err := srv.completeLink(t.Context(), entity.AuthMethodGithub, "ticket", claims,
		&entity.AuditMetadata{IP: "203.0.113.9"})

	require.ErrorIs(t, err, apperr.ErrProviderAlreadyConnected)
	// NOT wrapped in ErrLinkTicketUnusable: a conflict maps to link_conflict, and
	// rewrapping would send it to state_invalid instead.
	assert.NotErrorIs(t, err, apperr.ErrLinkTicketUnusable)

	rows := linkAuditRows(t, publisher.actions())
	require.Len(t, rows, 1, "a refused link publishes exactly one provider.linked row")
	require.NotNil(t, rows[0].Meta)
	assert.Equal(t, entity.AuditFailureLinkConflict, rows[0].Meta.FailureReason)

	// The row is provider.linked and nothing else: a login.failed carrying a
	// link-shaped reason would satisfy "a row appeared" while corrupting the
	// facet this action exists to keep clean.
	assert.Empty(t, loginFailureRows(publisher.actions()),
		"a link refusal must not be filed as a login failure")
}

// TestCompleteLinkAuditsSuccess pins criterion 21's success half.
//
// Deleting the publishLinked call was invisible to the suite: every link test
// asserted the redirect and the identity row, and none asked whether the trail
// recorded that an account gained a permanent new way in.
func TestCompleteLinkAuditsSuccess(t *testing.T) {
	t.Parallel()

	srv, _ := initServiceForMethod(t, entity.AuthMethodGithub)
	user := seedUser(t, srv)

	publisher := newRecordingAuditPublisher()
	srv.auditPublisher = publisher

	codes := mock_auth.NewMockDanceCodeStore(gomock.NewController(t))
	codes.EXPECT().ConsumeLinkTicket(gomock.Any(), "ticket").Return(&entity.LinkIntent{
		UserID:   user.ID,
		Provider: entity.AuthMethodGithub,
	}, nil)

	srv = srv.WithDance(config.Auth{}, codes)

	outcome, err := srv.completeLink(t.Context(), entity.AuthMethodGithub, "ticket",
		&entity.OAuthIDTokenClaims{
			Subject:       xuuid.NewString(),
			Email:         xuuid.NewString() + "@example.com",
			EmailVerified: true,
		},
		&entity.AuditMetadata{IP: "203.0.113.9"})

	require.NoError(t, err)
	require.True(t, outcome.Linked)

	rows := linkAuditRows(t, publisher.actions())
	require.Len(t, rows, 1)
	// No failure reason: that is what separates a success row from a refusal,
	// since both share one action.
	if rows[0].Meta != nil {
		assert.Empty(t, rows[0].Meta.FailureReason)
	}
}

// TestCompleteLinkRefusesOnARedeemStoreFailure is criterion 23's callback half.
//
// /start's half was already pinned; this one was not, and the mutation is the
// one the spec names outright: reading a store error as a miss. Here that would
// be worse than at /start -- the provider has already authenticated someone, so
// falling through mints a session for whichever account that was, on a request
// that asked to link.
func TestCompleteLinkRefusesOnARedeemStoreFailure(t *testing.T) {
	t.Parallel()

	srv, _ := initServiceForMethod(t, entity.AuthMethodGithub)

	codes := mock_auth.NewMockDanceCodeStore(gomock.NewController(t))
	codes.EXPECT().
		ConsumeLinkTicket(gomock.Any(), "ticket").
		Return(nil, errors.New("valkey is unreachable"))

	srv = srv.WithDance(config.Auth{}, codes)

	outcome, err := srv.completeLink(t.Context(), entity.AuthMethodGithub, "ticket",
		&entity.OAuthIDTokenClaims{Subject: "1", Email: "a@example.com", EmailVerified: true},
		&entity.AuditMetadata{IP: "203.0.113.9"})

	require.Error(t, err, "a store failure must refuse, never fall through")
	assert.Nil(t, outcome, "no outcome may be produced -- least of all a sign-in")
	assert.NotErrorIs(t, err, apperr.ErrLinkTicketUnusable,
		"an outage is not the same answer as a spent ticket")
}

// TestCompleteLinkRefusesASecondGithubIdentity is criterion 17.
//
// A DIFFERENT branch from criterion 15's: that one collides on the (provider,
// subject) pair, this one on (user, provider) -- the guard at link_identity.go
// that keeps one identity per provider per user, which is what makes the
// disconnect lockout check exact.
//
// Without a test here, deleting that guard would let one account accumulate
// several GitHub identities, and criteria 15 and 16 would both stay green.
func TestCompleteLinkRefusesASecondGithubIdentity(t *testing.T) {
	t.Parallel()

	srv, _ := initServiceForMethod(t, entity.AuthMethodGithub)
	user := seedUser(t, srv)

	// The account already holds a github identity, under some other subject.
	require.NoError(t, srv.usersSrv.LinkIdentity(t.Context(), user.ID, entity.AuthMethodGithub,
		&entity.OAuthIDTokenClaims{
			Subject:       xuuid.NewString(),
			Email:         xuuid.NewString() + "@example.com",
			EmailVerified: true,
		}))

	codes := mock_auth.NewMockDanceCodeStore(gomock.NewController(t))
	codes.EXPECT().ConsumeLinkTicket(gomock.Any(), "ticket").Return(&entity.LinkIntent{
		UserID:   user.ID,
		Provider: entity.AuthMethodGithub,
	}, nil)

	srv = srv.WithDance(config.Auth{}, codes)

	// A different GitHub account entirely -- a new subject, a new address.
	_, err := srv.completeLink(t.Context(), entity.AuthMethodGithub, "ticket",
		&entity.OAuthIDTokenClaims{
			Subject:       xuuid.NewString(),
			Email:         xuuid.NewString() + "@example.com",
			EmailVerified: true,
		},
		&entity.AuditMetadata{IP: "203.0.113.9"})

	require.ErrorIs(t, err, apperr.ErrProviderAlreadyConnected)
}

// danceableMethods makes one provider danceable without a registry row.
//
// Danceability is no longer a list read at boot: it is having a gateway in the
// live snapshot, which a unit test has no reloader to install. So the seam the
// service already exposes for this is used instead -- WithAuthMethods -- and
// only the two answers StartDance consults are overridden.
//
// The gateway is nil on purpose. These tests refuse BEFORE the gateway is
// touched, and a nil one is what proves it: were the refusal to move after the
// exchange, the test would panic rather than quietly pass.
type danceableMethods struct {
	inner  AuthMethods
	method entity.AuthMethod
}

func (m danceableMethods) Get(
	ctx context.Context, methodID entity.AuthMethod,
) (authmethod.AuthMethod, error) {
	return m.inner.Get(ctx, methodID)
}

func (m danceableMethods) Parse(name string) (entity.AuthMethod, bool) {
	return m.inner.Parse(name)
}

func (m danceableMethods) DanceProvider(segment string) (entity.AuthMethod, bool) {
	if entity.AuthMethod(segment) == m.method {
		return m.method, true
	}

	return m.inner.DanceProvider(segment)
}

func (m danceableMethods) DanceGateway(method entity.AuthMethod) (authmethod.Gateway, bool) {
	if method == m.method {
		return nil, true
	}

	return m.inner.DanceGateway(method)
}

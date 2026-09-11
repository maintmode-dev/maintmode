package auth

import (
	"context"
	"testing"

	"github.com/ruko1202/xlog"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap/zaptest"

	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

// TestSignInWithVerifiedClaims covers the method the OAuth dance calls directly,
// and which ExchangeIDToken now calls after verifying an id_token.
//
// It exists as an extraction rather than a copy because both paths run side by
// side in production: one identity must resolve to one user regardless of which
// path a person took, and a duplicated provisioning body is how that quietly
// stops being true.
func TestSignInWithVerifiedClaims(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	t.Run("mints a pair carrying a session id", func(t *testing.T) {
		t.Parallel()

		srv, _ := initService(t)

		claims := &entity.OAuthIDTokenClaims{
			Subject: xuuid.NewString(),
			Email:   xuuid.NewString() + "@example.com",
			Name:    "Dancer",
		}

		pair, user, err := srv.SignInWithVerifiedClaims(ctx, entity.AuthMethodGoogle, claims,
			entity.UserCreationPolicy{AllowCreate: true},
			&entity.AuditMetadata{IP: "10.0.0.9", UserAgent: "Mozilla/5.0"},
		)
		require.NoError(t, err)
		require.NotNil(t, user)
		require.NotEmpty(t, pair.AccessToken)
		require.NotEmpty(t, pair.RefreshToken)

		// SessionID ties the login to its audit row and is absent from the API
		// response DTO, so a caller serializing the wrong shape loses it
		// silently. It must be populated where it is minted.
		require.NotEqual(t, entity.TokenPair{}.SessionID, pair.SessionID)

		// The user is returned, not just the pair: the caller needs it to
		// publish LoginSuccess, which is why the signature carries three values.
		require.Equal(t, claims.Email, user.Email)
	})

	// The property that would break if the dance grew its own provisioning copy.
	t.Run("the same subject resolves to the same user", func(t *testing.T) {
		t.Parallel()

		srv, _ := initService(t)

		claims := &entity.OAuthIDTokenClaims{
			Subject: xuuid.NewString(),
			Email:   xuuid.NewString() + "@example.com",
			Name:    "Repeat",
		}

		_, first, err := srv.SignInWithVerifiedClaims(ctx, entity.AuthMethodGoogle, claims,
			entity.UserCreationPolicy{AllowCreate: true}, &entity.AuditMetadata{IP: "10.0.0.1"})
		require.NoError(t, err)

		_, second, err := srv.SignInWithVerifiedClaims(ctx, entity.AuthMethodGoogle, claims,
			entity.UserCreationPolicy{}, &entity.AuditMetadata{IP: "10.0.0.2"})
		require.NoError(t, err)

		require.Equal(t, first.ID, second.ID, "one identity must resolve to one user across both paths")
	})

	// The creation policy is a parameter precisely so the two callers can differ:
	// the BFF path derives AllowCreate from the dev-only X-Test-Roles header,
	// while a redirect arriving from Google carries no such header. Hard-coding
	// either choice inside the shared method would silently change the other.
	//
	// This asserts the policy is FORWARDED, not that it is decisive on its own:
	// GetOrCreateByAuthInfo resolves in the documented order
	// bootstrap > policy.AllowCreate > open signup > refuse, so on a stand with
	// no active admin the bootstrap branch creates the user whatever the policy
	// says. Asserting a refusal here would be asserting the test database's
	// state, not this method's contract.
	t.Run("forwards the creation policy rather than inventing one", func(t *testing.T) {
		t.Parallel()

		srv, _ := initService(t)

		claims := &entity.OAuthIDTokenClaims{
			Subject: xuuid.NewString(),
			Email:   xuuid.NewString() + "@example.com",
			Name:    "Granted",
		}

		_, user, err := srv.SignInWithVerifiedClaims(ctx, entity.AuthMethodGoogle, claims,
			entity.UserCreationPolicy{AllowCreate: true, GrantRoles: []entity.Role{entity.RoleEditor}},
			&entity.AuditMetadata{IP: "10.0.0.3"})
		require.NoError(t, err)
		require.Contains(t, user.Roles, entity.RoleEditor,
			"GrantRoles must reach GetOrCreateByAuthInfo; a dropped policy would silently change both callers")
	})
}

// TestExchangeIDTokenStillDerivesAllowCreateFromTestRoles is the AC7 regression
// guard for the extraction: the old path's dev-only auto-creation must survive
// unchanged, and it is exactly what a hard-coded policy inside the shared method
// would have removed.
func TestExchangeIDTokenStillDerivesAllowCreateFromTestRoles(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	srv, mocks := initService(t)

	claims := &entity.OAuthIDTokenClaims{
		Subject: xuuid.NewString(),
		Email:   xuuid.NewString() + "@example.com",
		Name:    "Tester",
	}
	mocks.authMethod.EXPECT().
		Authenticate(gomock.Any(), gomock.Any()).
		Return(claims, nil)

	pair, err := srv.ExchangeIDToken(ctx, &entity.ExchangeIDTokenCmd{
		Provider:  entity.AuthMethodGoogle,
		IDToken:   "tok",
		ClientIP:  "10.0.0.1",
		TestRoles: []entity.Role{entity.RoleAdmin},
	})
	require.NoError(t, err, "X-Test-Roles must still authorize creating an unknown user")
	require.NotEmpty(t, pair.AccessToken)
}

// TestSignInWithVerifiedClaims_AuditNamesTheMethod pins that an upstream sign-in
// is labeled, so that an empty method in the trail means "no credential was
// established", not "we forgot this path".
func TestSignInWithVerifiedClaims_AuditNamesTheMethod(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	t.Run("an upstream sign-in is labeled oidc", func(t *testing.T) {
		t.Parallel()

		srv, _ := initService(t)
		publisher := newRecordingAuditPublisher()
		srv.auditPublisher = publisher

		claims := &entity.OAuthIDTokenClaims{
			Subject: xuuid.NewString(),
			Email:   xuuid.NewString() + "@example.com",
			Name:    "Dancer",
		}

		_, _, err := srv.SignInWithVerifiedClaims(ctx, entity.AuthMethodGoogle, claims,
			entity.UserCreationPolicy{AllowCreate: true},
			&entity.AuditMetadata{IP: "10.0.0.9"},
		)
		require.NoError(t, err)

		actions := publisher.actions()
		require.NotEmpty(t, actions)
		success, ok := actions[len(actions)-1].(audit.LoginSuccess)
		require.True(t, ok, "expected a login success, got %T", actions[len(actions)-1])
		require.Equal(t, entity.AuditLoginMethodOIDC, success.Meta.LoginMethod)
	})

	// The caller's metadata must not be mutated: the same pointer reaches the
	// failure publish, and a method leaking across branches is the bug the
	// defensive copy in publishLoginFailure already exists to prevent.
	t.Run("the caller's metadata is left alone", func(t *testing.T) {
		t.Parallel()

		srv, _ := initService(t)
		srv.auditPublisher = newRecordingAuditPublisher()

		meta := &entity.AuditMetadata{IP: "10.0.0.11"}
		_, _, err := srv.SignInWithVerifiedClaims(ctx, entity.AuthMethodGoogle,
			&entity.OAuthIDTokenClaims{
				Subject: xuuid.NewString(),
				Email:   xuuid.NewString() + "@example.com",
				Name:    "Dancer",
			},
			entity.UserCreationPolicy{AllowCreate: true}, meta)
		require.NoError(t, err)

		require.Empty(t, meta.LoginMethod, "the caller's struct must not be written through")
	})
}

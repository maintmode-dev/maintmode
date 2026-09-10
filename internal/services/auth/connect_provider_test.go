package auth

import (
	"context"
	"testing"

	"github.com/ruko1202/xlog"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap/zaptest"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

func TestConnectProvider(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	t.Run("ok - verifies token and links identity", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initService(t)

		// Seed a user whose only identity is github, so connecting google adds a
		// new provider (one identity per provider).
		user, err := srv.usersSrv.GetOrCreateByAuthInfo(ctx, entity.AuthMethodGithub, &entity.OAuthProviderUserInfo{
			ID:    xuuid.NewString(),
			Email: xuuid.NewString() + "@example.com",
			Name:  "User",
		}, entity.UserCreationPolicy{AllowCreate: true})
		require.NoError(t, err)

		// Connect google (the mock provider resolves to google).
		connectClaims := &entity.OAuthIDTokenClaims{
			Subject: xuuid.NewString(),
			Email:   xuuid.NewString() + "@example.com",
			Name:    "User",
		}
		mocks.authMethod.EXPECT().
			Authenticate(gomock.Any(), "id-token").
			Return(connectClaims, nil)

		err = srv.ConnectProvider(ctx, &entity.ConnectProviderCmd{
			UserID:   user.ID,
			Provider: string(entity.AuthMethodGoogle),
			IDToken:  "id-token",
		})
		require.NoError(t, err)
	})

	t.Run("invalid id token", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initService(t)

		mocks.authMethod.EXPECT().
			Authenticate(gomock.Any(), gomock.Any()).
			Return(nil, apperr.ErrInvalidAccessToken)

		err := srv.ConnectProvider(ctx, &entity.ConnectProviderCmd{
			UserID:   xuuid.New(),
			Provider: string(entity.AuthMethodGoogle),
			IDToken:  "bad",
		})
		require.ErrorIs(t, err, apperr.ErrInvalidAccessToken)
	})

	t.Run("unsupported provider", func(t *testing.T) {
		t.Parallel()

		srv, _ := initService(t)

		err := srv.ConnectProvider(ctx, &entity.ConnectProviderCmd{
			UserID:   xuuid.New(),
			Provider: string(entity.AuthMethodGithub),
			IDToken:  "tok",
		})
		require.ErrorIs(t, err, apperr.ErrUnsupportedProvider)
	})

	// The reserved names are refused even when they ARE registered, which is
	// the distinction the case above cannot make: github fails merely by being
	// absent from the registry, and would keep failing with no gate at all.
	//
	// stub accepts any credential and mints an identity, so a client naming it
	// is a client asking to skip verification; bootstrap resolves an identity
	// from configured email and grants admin past the seats cap, which is safe
	// only behind the break-glass secret. Either one linked through /me would
	// attach an attacker-chosen identity to their own account.
	for _, reserved := range []entity.AuthMethod{entity.AuthMethodStub, entity.AuthMethodBootstrap} {
		t.Run("refuses the reserved name "+string(reserved)+" even when registered", func(t *testing.T) {
			t.Parallel()

			srv, mocks := initServiceForMethod(t, reserved)

			// Times(0): the refusal must happen BEFORE the credential is looked
			// at. A gate that verified first and refused after would satisfy a
			// plain error assertion while still handing the token to a provider
			// that accepts anything.
			mocks.authMethod.EXPECT().Authenticate(gomock.Any(), gomock.Any()).Times(0)

			err := srv.ConnectProvider(ctx, &entity.ConnectProviderCmd{
				UserID:   xuuid.New(),
				Provider: string(reserved),
				IDToken:  "tok",
			})
			require.ErrorIs(t, err, apperr.ErrUnsupportedProvider)
		})
	}
}

// TestDisconnectProviderIgnoresAnUnlinkedProvider pins that a name the user has
// not linked succeeds without opening a transaction: the account is already in
// the requested state.
//
// The user here has exactly ONE identity, which is what gives the test teeth.
// Without the membership check, UnlinkIdentity takes FOR UPDATE on the user row
// and hits the last-provider guard before it reaches the delete -- so every name
// below would come back as "cannot disconnect your last sign-in method", about a
// provider that never existed. The single remaining identity below proves
// nothing was deleted along the way.
func TestDisconnectProviderIgnoresAnUnlinkedProvider(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	srv, _ := initService(t)

	user, err := srv.usersSrv.GetOrCreateByAuthInfo(ctx, entity.AuthMethodGoogle, &entity.OAuthProviderUserInfo{
		ID:    xuuid.NewString(),
		Email: xuuid.NewString() + "@example.com",
		Name:  "Disconnect Probe",
	}, entity.UserCreationPolicy{AllowCreate: true})
	require.NoError(t, err)

	for _, name := range []string{"", "acme", "../../etc/passwd", "a b"} {
		err := srv.DisconnectProvider(ctx, &entity.DisconnectProviderCmd{
			UserID:   user.ID,
			Provider: name,
		})
		require.NoError(t, err, "name %q", name)
	}

	linked, err := srv.usersSrv.ListConnectedProviders(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, []entity.AuthMethod{entity.AuthMethodGoogle}, linked)
}

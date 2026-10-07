package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xcripto"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

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

// TestDisconnectProvider_LastProviderNeedsAWorkingBuiltin walks the rule for a
// user whose only provider is Google: it may go exactly when a built-in method
// would still let them in. Each case changes one input, so a guard that ignored
// any of the three -- the password, either flag -- fails one of them. The
// "password set, both methods off" case is the lockout: a password the
// instance no longer accepts is not a way in.
func TestDisconnectProvider_LastProviderNeedsAWorkingBuiltin(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		passwordSet bool
		flags       map[entity.AuthMethodName]bool
		allowed     bool
	}{
		"password set, password sign-in on": {
			passwordSet: true,
			flags:       map[entity.AuthMethodName]bool{entity.AuthMethodNameEmailPassword: true},
			allowed:     true,
		},
		"password set, both methods off": {
			passwordSet: true,
			flags:       map[entity.AuthMethodName]bool{},
			allowed:     false,
		},
		"no password, password sign-in on": {
			flags:   map[entity.AuthMethodName]bool{entity.AuthMethodNameEmailPassword: true},
			allowed: false,
		},
		"no password, code sign-in on": {
			flags:   map[entity.AuthMethodName]bool{entity.AuthMethodNameEmailOTP: true},
			allowed: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			srv, _ := initServiceWithUnrelatedBootstrapFlags(t, flagsWith(tc.flags))

			user, err := srv.usersSrv.GetOrCreateByAuthInfo(ctx, entity.AuthMethodGoogle, &entity.OAuthProviderUserInfo{
				ID:    xuuid.NewString(),
				Email: xuuid.NewString() + "@example.com",
				Name:  "Last Provider Probe",
			}, entity.UserCreationPolicy{AllowCreate: true})
			require.NoError(t, err)

			if tc.passwordSet {
				hash, err := xcripto.HashPassword("correct-horse-" + xuuid.NewString())
				require.NoError(t, err)
				require.NoError(t, srv.passwords.UpsertPassword(ctx, user.ID, hash))
			}

			err = srv.DisconnectProvider(ctx, &entity.DisconnectProviderCmd{
				UserID:   user.ID,
				Provider: string(entity.AuthMethodGoogle),
			})

			linked, listErr := srv.usersSrv.ListConnectedProviders(ctx, user.ID)
			require.NoError(t, listErr)

			if tc.allowed {
				require.NoError(t, err)
				require.Empty(t, linked)
			} else {
				require.ErrorIs(t, err, apperr.ErrCannotDisconnectLastProvider)
				require.Equal(t, []entity.AuthMethod{entity.AuthMethodGoogle}, linked)
			}
		})
	}
}

// A user with another provider linked keeps a way in whatever the built-in
// flags say, so the disconnect must not depend on reading them: a flag store
// that cannot answer must not fail a disconnect that never needed it.
func TestDisconnectProvider_NonLastProviderDoesNotReadTheFlags(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	srv, _ := initServiceWithUnrelatedBootstrapFlags(t, stubMethodFlags{fail: true})

	user, err := srv.usersSrv.GetOrCreateByAuthInfo(ctx, entity.AuthMethodGoogle, &entity.OAuthProviderUserInfo{
		ID:    xuuid.NewString(),
		Email: xuuid.NewString() + "@example.com",
		Name:  "Two Providers Probe",
	}, entity.UserCreationPolicy{AllowCreate: true})
	require.NoError(t, err)
	require.NoError(t, srv.usersSrv.LinkIdentity(ctx, user.ID, entity.AuthMethodGithub, &entity.OAuthIDTokenClaims{
		Subject: "gh-" + xuuid.NewString(),
		Email:   xuuid.NewString() + "@example.com",
		Name:    "GH",
	}))

	require.NoError(t, srv.DisconnectProvider(ctx, &entity.DisconnectProviderCmd{
		UserID:   user.ID,
		Provider: string(entity.AuthMethodGithub),
	}))

	linked, err := srv.usersSrv.ListConnectedProviders(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, []entity.AuthMethod{entity.AuthMethodGoogle}, linked)
}

// The last provider is the one disconnect that reads the flags, and a flag
// that cannot be read must refuse it rather than guess. Guessing "offered"
// would leave the account with no way in whenever the settings store fails;
// answering "last provider" would tell the user something untrue about why.
func TestDisconnectProvider_UnreadableFlagsKeepTheLastProvider(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	srv, _ := initServiceWithUnrelatedBootstrapFlags(t, stubMethodFlags{fail: true})

	user, err := srv.usersSrv.GetOrCreateByAuthInfo(ctx, entity.AuthMethodGoogle, &entity.OAuthProviderUserInfo{
		ID:    xuuid.NewString(),
		Email: xuuid.NewString() + "@example.com",
		Name:  "Unreadable Flags Probe",
	}, entity.UserCreationPolicy{AllowCreate: true})
	require.NoError(t, err)

	err = srv.DisconnectProvider(ctx, &entity.DisconnectProviderCmd{
		UserID:   user.ID,
		Provider: string(entity.AuthMethodGoogle),
	})
	require.Error(t, err)
	require.NotErrorIs(t, err, apperr.ErrCannotDisconnectLastProvider,
		"an unreadable flag is a failure, not a verdict on the account")

	linked, err := srv.usersSrv.ListConnectedProviders(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, []entity.AuthMethod{entity.AuthMethodGoogle}, linked)
}

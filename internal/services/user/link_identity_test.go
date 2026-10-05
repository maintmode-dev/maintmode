package user

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

func claimsFor(email string) *entity.OAuthIDTokenClaims {
	return &entity.OAuthIDTokenClaims{
		Subject: xuuid.NewString(),
		Email:   email,
		Name:    "Linked User",
	}
}

func TestLinkIdentity(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	srv := initService(t)

	t.Run("ok - links a new provider", func(t *testing.T) {
		t.Parallel()

		user := makeUser(ctx, t, srv)

		err := srv.LinkIdentity(ctx, user.ID, entity.AuthMethodGithub, claimsFor("gh-"+xuuid.NewString()+"@example.com"))
		require.NoError(t, err)

		providers, err := srv.ListConnectedProviders(ctx, user.ID)
		require.NoError(t, err)
		require.Contains(t, providers, entity.AuthMethodGoogle)
		require.Contains(t, providers, entity.AuthMethodGithub)
	})

	// Its way in is the break-glass password; a linked provider would outlive a
	// change of that password.
	t.Run("the break-glass account cannot link a provider", func(t *testing.T) {
		t.Parallel()

		breakGlass, err := srv.EnsureBreakGlassAccount(ctx, breakGlassEmail(), "Break-glass admin")
		require.NoError(t, err)

		err = srv.LinkIdentity(ctx, breakGlass.ID, entity.AuthMethodGithub, claimsFor("gh-"+xuuid.NewString()+"@example.com"))
		require.ErrorIs(t, err, apperr.ErrBreakGlassPersonalSignIn)

		providers, err := srv.ListConnectedProviders(ctx, breakGlass.ID)
		require.NoError(t, err)
		require.NotContains(t, providers, entity.AuthMethodGithub)
	})

	t.Run("already connected to this user", func(t *testing.T) {
		t.Parallel()

		user := makeUser(ctx, t, srv)
		claims := claimsFor("gh-" + xuuid.NewString() + "@example.com")

		require.NoError(t, srv.LinkIdentity(ctx, user.ID, entity.AuthMethodGithub, claims))

		err := srv.LinkIdentity(ctx, user.ID, entity.AuthMethodGithub, claims)
		require.ErrorIs(t, err, apperr.ErrProviderAlreadyConnected)
	})

	t.Run("same provider, different subject, same user -> already connected", func(t *testing.T) {
		t.Parallel()

		user := makeUser(ctx, t, srv)

		require.NoError(t, srv.LinkIdentity(ctx, user.ID, entity.AuthMethodGithub, claimsFor("gh-a-"+xuuid.NewString()+"@example.com")))

		// A second github identity under a different subject must be rejected so
		// the user keeps at most one identity per provider.
		err := srv.LinkIdentity(ctx, user.ID, entity.AuthMethodGithub, claimsFor("gh-b-"+xuuid.NewString()+"@example.com"))
		require.ErrorIs(t, err, apperr.ErrProviderAlreadyConnected)

		providers, err := srv.ListConnectedProviders(ctx, user.ID)
		require.NoError(t, err)
		require.ElementsMatch(t, []entity.AuthMethod{entity.AuthMethodGoogle, entity.AuthMethodGithub}, providers)
	})

	t.Run("linked to another user", func(t *testing.T) {
		t.Parallel()

		owner := makeUser(ctx, t, srv)
		other := makeUser(ctx, t, srv)
		claims := claimsFor("gh-" + xuuid.NewString() + "@example.com")

		require.NoError(t, srv.LinkIdentity(ctx, owner.ID, entity.AuthMethodGithub, claims))

		err := srv.LinkIdentity(ctx, other.ID, entity.AuthMethodGithub, claims)
		require.ErrorIs(t, err, apperr.ErrProviderLinkedToAnotherUser)
	})
}

func TestUnlinkIdentity(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	srv := initService(t)

	t.Run("ok - removes a non-last provider", func(t *testing.T) {
		t.Parallel()

		user := makeUser(ctx, t, srv)
		require.NoError(t, srv.LinkIdentity(ctx, user.ID, entity.AuthMethodGithub, claimsFor("gh-"+xuuid.NewString()+"@example.com")))

		err := srv.UnlinkIdentity(ctx, user.ID, entity.AuthMethodGithub, false)
		require.NoError(t, err)

		providers, err := srv.ListConnectedProviders(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, []entity.AuthMethod{entity.AuthMethodGoogle}, providers)
	})

	t.Run("lockout - cannot disconnect the only provider", func(t *testing.T) {
		t.Parallel()

		user := makeUser(ctx, t, srv)

		err := srv.UnlinkIdentity(ctx, user.ID, entity.AuthMethodGoogle, false)
		require.ErrorIs(t, err, apperr.ErrCannotDisconnectLastProvider)

		providers, err := srv.ListConnectedProviders(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, []entity.AuthMethod{entity.AuthMethodGoogle}, providers)
	})

	t.Run("ok - the last provider goes when a built-in method keeps the user in", func(t *testing.T) {
		t.Parallel()

		user := makeUser(ctx, t, srv)

		require.NoError(t, srv.UnlinkIdentity(ctx, user.ID, entity.AuthMethodGoogle, true))

		providers, err := srv.ListConnectedProviders(ctx, user.ID)
		require.NoError(t, err)
		require.Empty(t, providers)
	})
}

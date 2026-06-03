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

		err := srv.LinkIdentity(ctx, user.ID, entity.OAuthProviderGithub, claimsFor("gh-"+xuuid.NewString()+"@example.com"))
		require.NoError(t, err)

		providers, err := srv.ListConnectedProviders(ctx, user.ID)
		require.NoError(t, err)
		require.Contains(t, providers, entity.OAuthProviderGoogle)
		require.Contains(t, providers, entity.OAuthProviderGithub)
	})

	t.Run("already connected to this user", func(t *testing.T) {
		t.Parallel()

		user := makeUser(ctx, t, srv)
		claims := claimsFor("gh-" + xuuid.NewString() + "@example.com")

		require.NoError(t, srv.LinkIdentity(ctx, user.ID, entity.OAuthProviderGithub, claims))

		err := srv.LinkIdentity(ctx, user.ID, entity.OAuthProviderGithub, claims)
		require.ErrorIs(t, err, apperr.ErrProviderAlreadyConnected)
	})

	t.Run("same provider, different subject, same user -> already connected", func(t *testing.T) {
		t.Parallel()

		user := makeUser(ctx, t, srv)

		require.NoError(t, srv.LinkIdentity(ctx, user.ID, entity.OAuthProviderGithub, claimsFor("gh-a-"+xuuid.NewString()+"@example.com")))

		// A second github identity under a different subject must be rejected so
		// the user keeps at most one identity per provider.
		err := srv.LinkIdentity(ctx, user.ID, entity.OAuthProviderGithub, claimsFor("gh-b-"+xuuid.NewString()+"@example.com"))
		require.ErrorIs(t, err, apperr.ErrProviderAlreadyConnected)

		providers, err := srv.ListConnectedProviders(ctx, user.ID)
		require.NoError(t, err)
		require.ElementsMatch(t, []entity.OAuthProvider{entity.OAuthProviderGoogle, entity.OAuthProviderGithub}, providers)
	})

	t.Run("linked to another user", func(t *testing.T) {
		t.Parallel()

		owner := makeUser(ctx, t, srv)
		other := makeUser(ctx, t, srv)
		claims := claimsFor("gh-" + xuuid.NewString() + "@example.com")

		require.NoError(t, srv.LinkIdentity(ctx, owner.ID, entity.OAuthProviderGithub, claims))

		err := srv.LinkIdentity(ctx, other.ID, entity.OAuthProviderGithub, claims)
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
		require.NoError(t, srv.LinkIdentity(ctx, user.ID, entity.OAuthProviderGithub, claimsFor("gh-"+xuuid.NewString()+"@example.com")))

		err := srv.UnlinkIdentity(ctx, user.ID, entity.OAuthProviderGithub)
		require.NoError(t, err)

		providers, err := srv.ListConnectedProviders(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, []entity.OAuthProvider{entity.OAuthProviderGoogle}, providers)
	})

	t.Run("lockout - cannot disconnect the only provider", func(t *testing.T) {
		t.Parallel()

		user := makeUser(ctx, t, srv)

		err := srv.UnlinkIdentity(ctx, user.ID, entity.OAuthProviderGoogle)
		require.ErrorIs(t, err, apperr.ErrCannotDisconnectLastProvider)

		providers, err := srv.ListConnectedProviders(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, []entity.OAuthProvider{entity.OAuthProviderGoogle}, providers)
	})

	t.Run("not connected provider is a no-op success", func(t *testing.T) {
		t.Parallel()

		user := makeUser(ctx, t, srv)
		// Link a second provider so the lockout guard passes; disconnecting a
		// provider the user never linked must succeed without changing anything.
		require.NoError(t, srv.LinkIdentity(ctx, user.ID, entity.OAuthProviderGithub, claimsFor("gh-"+xuuid.NewString()+"@example.com")))

		err := srv.UnlinkIdentity(ctx, user.ID, entity.OAuthProviderStub)
		require.NoError(t, err)

		providers, err := srv.ListConnectedProviders(ctx, user.ID)
		require.NoError(t, err)
		require.ElementsMatch(t, []entity.OAuthProvider{entity.OAuthProviderGoogle, entity.OAuthProviderGithub}, providers)
	})
}

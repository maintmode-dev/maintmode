package entity_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

func TestPrimaryOAuthProvider(t *testing.T) {
	t.Parallel()

	t.Run("returns the first provider when present", func(t *testing.T) {
		t.Parallel()
		got := entity.PrimaryOAuthProvider([]entity.OAuthProvider{
			entity.OAuthProviderGithub,
			entity.OAuthProviderGoogle,
		})
		require.Equal(t, entity.OAuthProviderGithub, got)
	})

	t.Run("falls back to unknown for empty input", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, entity.OAuthProviderUnknown, entity.PrimaryOAuthProvider(nil))
		require.Equal(t, entity.OAuthProviderUnknown, entity.PrimaryOAuthProvider([]entity.OAuthProvider{}))
	})
}

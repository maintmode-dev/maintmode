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

func TestParseOAuthProvider(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  entity.OAuthProvider
		ok    bool
	}{
		{name: "google is a login provider", input: "google", want: entity.OAuthProviderGoogle, ok: true},
		{name: "github is a login provider", input: "github", want: entity.OAuthProviderGithub, ok: true},
		// The stub accepts any token and mints an identity. The provider name
		// arrives straight from the accept-invitation request body, so it is
		// refused at the API boundary in every environment (RUK-249).
		{name: "stub is refused", input: "stub", ok: false},
		{name: "unknown is output-only", input: "unknown", ok: false},
		{name: "empty is refused", input: "", ok: false},
		// Matching is exact: no case folding that could smuggle the stub back in.
		{name: "casing is not folded", input: "STUB", ok: false},
		{name: "google casing is not folded", input: "Google", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := entity.ParseOAuthProvider(tt.input)
			require.Equal(t, tt.ok, ok)
			require.Equal(t, tt.want, got)
		})
	}
}

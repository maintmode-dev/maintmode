package entity_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

// TestAuthMethodStringValues pins the constants against string LITERALS, which
// nothing else in the repo does.
//
// It has to be its own test rather than a corollary of the parser table:
// "stub" and "unknown" appear there only as rejected inputs, so renaming
// AuthMethodStub to "stubbed" would leave every parser case green — the string
// "stub" is still refused, just for a different reason. The /me and users-list
// tests look like literal pins but are not either: they read
// require.Equal(t, string(entity.AuthMethodGoogle), got.OAuthProvider), the
// same constant on both sides.
//
// These values are written to user_identities.provider, a TEXT column with no
// CHECK constraint. A changed literal would not fail — it would silently stop
// matching existing rows, orphaning every linked identity.
func TestAuthMethodStringValues(t *testing.T) {
	t.Parallel()

	require.Equal(t, "google", string(entity.AuthMethodGoogle))
	require.Equal(t, "github", string(entity.AuthMethodGithub))
	require.Equal(t, "stub", string(entity.AuthMethodStub))
	require.Equal(t, "unknown", string(entity.AuthMethodUnknown))
	require.Equal(t, "email", string(entity.AuthMethodEmail))
	require.Equal(t, "bootstrap", string(entity.AuthMethodBootstrap))
}

func TestPrimaryAuthMethod(t *testing.T) {
	t.Parallel()

	t.Run("returns the first method when present", func(t *testing.T) {
		t.Parallel()
		got := entity.PrimaryAuthMethod([]entity.AuthMethod{
			entity.AuthMethodGithub,
			entity.AuthMethodGoogle,
		})
		require.Equal(t, entity.AuthMethodGithub, got)
	})

	t.Run("falls back to unknown for empty input", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, entity.AuthMethodUnknown, entity.PrimaryAuthMethod(nil))
		require.Equal(t, entity.AuthMethodUnknown, entity.PrimaryAuthMethod([]entity.AuthMethod{}))
	})
}

func TestParseAuthMethod(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  entity.AuthMethod
		ok    bool
	}{
		{name: "google is a login method", input: "google", want: entity.AuthMethodGoogle, ok: true},
		{name: "github is a login method", input: "github", want: entity.AuthMethodGithub, ok: true},
		// Accepted by the parser, but no implementation stands behind them yet:
		// Methods.Get misses the registry and the caller refuses the request
		// (RUK-283). Tasks 2/13 onward add the implementations.
		{name: "email is accepted with no implementation", input: "email", want: entity.AuthMethodEmail, ok: true},
		{name: "bootstrap is accepted with no implementation", input: "bootstrap", want: entity.AuthMethodBootstrap, ok: true},
		// The stub accepts any token and mints an identity. The method name
		// arrives straight from the accept-invitation request body, so it is
		// refused at the API boundary in every environment (RUK-249).
		{name: "stub is refused", input: "stub", ok: false},
		{name: "unknown is output-only", input: "unknown", ok: false},
		{name: "empty is refused", input: "", ok: false},
		// Matching is exact: no case folding that could smuggle the stub back in.
		{name: "casing is not folded", input: "STUB", ok: false},
		{name: "google casing is not folded", input: "Google", ok: false},
		{name: "email casing is not folded", input: "EMAIL", ok: false},
		{name: "bootstrap casing is not folded", input: "Bootstrap", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := entity.ParseAuthMethod(tt.input)
			require.Equal(t, tt.ok, ok)
			require.Equal(t, tt.want, got)
		})
	}
}

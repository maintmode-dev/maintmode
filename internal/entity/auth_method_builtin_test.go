package entity_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

// TestIsBuiltin covers the set that decides which column an identity is
// written to.
//
// The vocabulary lives here rather than in a CHECK constraint, matching
// integration_settings.kind and entity.AuthMethodName: which methods exist is a
// fact about the code. So this test is the enforcement, not a mirror of one --
// the database accepts whatever string it is handed.
//
// What is NOT built in matters as much as what is. A provider misclassified
// here would be stored by name in builtin_method and never resolved against the
// registry, which is precisely the silent demotion the sign-in path refuses to
// make when a lookup fails.
func TestIsBuiltin(t *testing.T) {
	t.Parallel()

	t.Run("break-glass authenticates without a registry row", func(t *testing.T) {
		t.Parallel()
		require.True(t, entity.AuthMethodBootstrap.IsBuiltin())
	})

	t.Run("configured providers resolve against the registry", func(t *testing.T) {
		t.Parallel()

		for _, method := range []entity.AuthMethod{
			entity.AuthMethodGoogle,
			entity.AuthMethodGithub,
			// The dev stub is NOT built in: use_stub substitutes it inside
			// Methods.Get and the caller keeps the original name, so an identity
			// on such a stand is written against that provider's registry row.
			entity.AuthMethodStub,
			// Declared but unimplemented; it writes no identity at all today.
			entity.AuthMethodEmail,
		} {
			require.False(t, method.IsBuiltin(), "%s must resolve against the registry", method)
		}
	})

	t.Run("an unknown name is not built in", func(t *testing.T) {
		t.Parallel()
		require.False(t, entity.AuthMethod("password").IsBuiltin())
		require.False(t, entity.AuthMethodUnknown.IsBuiltin())
	})
}

package user

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/storages/users"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

// TestSignIn_UnresolvableProviderIsUnsupported covers the window where a method
// is still in the auth snapshot but its registry row is gone.
//
// It is a normal window, not a corrupt state: the reloader rebuilds
// asynchronously, and on a failed rebuild it deliberately keeps the previous
// snapshot live -- so a deleted provider stays listed until the next successful
// pass. A user holding that button must not be told the thing they asked for
// does not exist (404 speaks to an admin reading about an integration); they
// named a provider the instance was advertising a moment ago.
//
// The other half matters just as much: the sign-in must FAIL. Falling through
// to the built-in branch would create an account with no provider behind it.
func TestSignIn_UnresolvableProviderIsUnsupported(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv, _ := initPolicyService(t, users.NewStore(db), true)

	// A name the resolver was never given, standing in for a row deleted since
	// the snapshot was built.
	vanished := entity.AuthMethod("vanished-" + xuuid.NewString())

	user, err := srv.GetOrCreateByAuthInfo(ctx, vanished, oauthInfo(),
		entity.UserCreationPolicy{AllowCreate: true})

	require.ErrorIs(t, err, apperr.ErrUnsupportedProvider,
		"a missing registry row is a 400, not the 404 the integration lookup would give")
	require.NotErrorIs(t, err, apperr.ErrIntegrationNotFound,
		"the integration-facing error must not reach a signing-in user")
	require.Nil(t, user, "no account may be created behind a provider that does not exist")
}

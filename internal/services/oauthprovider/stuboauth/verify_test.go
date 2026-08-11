package stuboauth_test

import (
	"context"
	"strings"
	"testing"

	"github.com/ruko1202/xlog"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/services/oauthprovider/stuboauth"
)

func TestServiceProviderID(t *testing.T) {
	t.Parallel()

	require.Equal(
		t,
		entity.OAuthProviderStub,
		stuboauth.NewService().ProviderID(),
	)
}

func TestServiceVerifyToken(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	t.Run("accepts any token and synthesizes claims", func(t *testing.T) {
		t.Parallel()

		claims, err := stuboauth.NewService().VerifyToken(ctx, "anything-at-all")
		require.NoError(t, err)
		require.NotNil(t, claims)
		require.NotEmpty(t, claims.Subject)
		require.NotEmpty(t, claims.Email)
		require.NotEmpty(t, claims.Name)
		require.True(
			t,
			strings.HasSuffix(claims.Email, "@mail.com"),
			"stub email %q should use the @mail.com domain", claims.Email,
		)
	})

	t.Run("empty token is still accepted", func(t *testing.T) {
		t.Parallel()

		// The stub only rejects one hardcoded sentinel; everything else, including
		// the empty string, is waved through. This documents that the stub performs
		// no validation whatsoever, which is why it is gated on dev.
		claims, err := stuboauth.NewService().VerifyToken(ctx, "")
		require.NoError(t, err)
		require.NotEmpty(t, claims.Subject)
	})

	t.Run("rejects the sentinel invalid token", func(t *testing.T) {
		t.Parallel()

		claims, err := stuboauth.NewService().VerifyToken(ctx, "this-is-not-a-valid-jwt")
		require.Nil(t, claims)
		require.ErrorIs(t, err, apperr.ErrInvalidAccessToken)
	})

	t.Run("sentinel match is exact", func(t *testing.T) {
		t.Parallel()

		srv := stuboauth.NewService()

		// Guards against the rejection being loosened to a prefix/substring match.
		for _, token := range []string{
			"this-is-not-a-valid-jwt ",
			" this-is-not-a-valid-jwt",
			"this-is-not-a-valid-jwt-really",
			"This-Is-Not-A-Valid-Jwt",
		} {
			claims, err := srv.VerifyToken(ctx, token)
			require.NoError(t, err, "token %q should not match the sentinel", token)
			require.NotNil(t, claims)
		}
	})

	t.Run("each call mints a fresh identity", func(t *testing.T) {
		t.Parallel()

		srv := stuboauth.NewService()

		first, err := srv.VerifyToken(ctx, "token")
		require.NoError(t, err)
		second, err := srv.VerifyToken(ctx, "token")
		require.NoError(t, err)

		// The stub is not a fake user store: the same token yields a different user
		// every time. Anything relying on stub logins being idempotent would break.
		require.NotEqual(t, first.Subject, second.Subject)
		require.NotEqual(t, first.Email, second.Email)
	})

	t.Run("subject is not reused as the email local part", func(t *testing.T) {
		t.Parallel()

		claims, err := stuboauth.NewService().VerifyToken(ctx, "token")
		require.NoError(t, err)

		// verify.go draws two independent UUIDs: `id` feeds the email and name,
		// while Subject gets its own. The email/name therefore share an id with each
		// other but not with Subject.
		local, _, found := strings.Cut(claims.Email, "@")
		require.True(t, found)
		require.NotEqual(t, claims.Subject, local)
		require.Equal(t, "User Name["+local+"]", claims.Name)
	})
}

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

	t.Run("an email token becomes the identity", func(t *testing.T) {
		t.Parallel()

		claims, err := stuboauth.NewService().VerifyToken(ctx, "invited@example.com")
		require.NoError(t, err)

		// The strict invitation email check now runs in dev too, so a dev stand
		// can only accept an invitation if the stub returns the invited address
		// rather than a random one.
		require.Equal(t, "invited@example.com", claims.Email)
		require.Equal(t, "User Name[invited]", claims.Name)
		require.NotEmpty(t, claims.Subject)
	})

	t.Run("an email token yields the same identity every time", func(t *testing.T) {
		t.Parallel()

		srv := stuboauth.NewService()

		first, err := srv.VerifyToken(ctx, "repeat@example.com")
		require.NoError(t, err)
		second, err := srv.VerifyToken(ctx, "repeat@example.com")
		require.NoError(t, err)

		// Subject keys the user record, so a random one would create a second
		// user on every dev login instead of signing the same one back in.
		require.Equal(t, first.Subject, second.Subject)
		require.Equal(t, first.Email, second.Email)
	})

	t.Run("subject of an email token is case-insensitive", func(t *testing.T) {
		t.Parallel()

		srv := stuboauth.NewService()

		lower, err := srv.VerifyToken(ctx, "casing@example.com")
		require.NoError(t, err)
		upper, err := srv.VerifyToken(ctx, "Casing@Example.com")
		require.NoError(t, err)

		// Email comparison upstream is case-insensitive, so the identity keyed
		// off it must be too — otherwise the same person gets two user records.
		require.Equal(t, lower.Subject, upper.Subject)
		// The address itself is echoed verbatim; only the subject is normalized.
		require.Equal(t, "Casing@Example.com", upper.Email)
	})

	t.Run("all three claims describe the same address", func(t *testing.T) {
		t.Parallel()

		srv := stuboauth.NewService()

		// mail.ParseAddress accepts more than a bare address, and Email is what
		// lands in users.email and is compared against the invitation. Echoing
		// the raw token there would let Subject and Email describe different
		// people — a leading space alone produced a mismatch the operator sees
		// as an unexplained email_mismatch.
		for _, token := range []string{
			" padded@example.com",
			"padded@example.com ",
			"Display Name <padded@example.com>",
			"<padded@example.com>",
		} {
			claims, err := srv.VerifyToken(ctx, token)
			require.NoError(t, err, "token %q", token)
			require.Equal(t, "padded@example.com", claims.Email, "token %q", token)
			require.Equal(t, "User Name[padded]", claims.Name, "token %q", token)
		}

		// And the identity is the same one the bare address resolves to.
		bare, err := srv.VerifyToken(ctx, "padded@example.com")
		require.NoError(t, err)
		spaced, err := srv.VerifyToken(ctx, " padded@example.com")
		require.NoError(t, err)
		require.Equal(t, bare.Subject, spaced.Subject)
	})

	t.Run("a non-email token keeps the random identity", func(t *testing.T) {
		t.Parallel()

		srv := stuboauth.NewService()

		// The API suite logs in with bare non-email strings and relies on each
		// call minting a fresh user; only a parseable address opts into the
		// deterministic branch.
		for _, token := range []string{"api-test-seed-approver", "token", "", "not@an@address"} {
			first, err := srv.VerifyToken(ctx, token)
			require.NoError(t, err, "token %q", token)
			second, err := srv.VerifyToken(ctx, token)
			require.NoError(t, err, "token %q", token)

			require.NotEqual(t, first.Subject, second.Subject, "token %q", token)
			require.True(
				t,
				strings.HasSuffix(first.Email, "@mail.com"),
				"token %q should keep the synthetic domain, got %q", token, first.Email,
			)
		}
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

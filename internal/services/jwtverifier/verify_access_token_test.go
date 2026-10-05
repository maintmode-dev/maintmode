package jwtverifier

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/ruko1202/xlog"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/utils/xtime"

	"github.com/ruko1202/maintmode/internal/entity"
)

const testIssuer = "oauth-service"

func TestVerifyAccessToken(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		kid := "kid-1"

		key := newTestKey(t)

		verifier := newTestVerifier(ctx, t, key, kid)
		token := signTestToken(t, key, kid, testIssuer, time.Now().Add(time.Hour), []entity.Role{entity.RoleEditor})

		claims, err := verifier.VerifyAccessToken(ctx, token)
		require.NoError(t, err)
		require.Equal(t, "alice@example.com", claims.UserEmail)
		require.Equal(t, []entity.Role{entity.RoleEditor}, claims.UserRoles)
		require.NotEmpty(t, claims.Subject)
	})

	t.Run("errors", func(t *testing.T) {
		t.Parallel()

		key := newTestKey(t)
		kid := "kid-1"
		verifier := newTestVerifier(ctx, t, key, kid)

		tests := []struct {
			name      string
			token     string
			targetErr error
		}{
			{
				name:      "expired",
				token:     signTestToken(t, key, kid, testIssuer, xtime.UTCNow().Add(-time.Hour), []entity.Role{entity.RoleGuest}),
				targetErr: apperr.ErrTokenExpired,
			},
			{
				name:      "wrong issuer",
				token:     signTestToken(t, key, kid, "other-issuer", xtime.UTCNow().Add(time.Hour), []entity.Role{entity.RoleGuest}),
				targetErr: apperr.ErrInvalidAccessToken,
			},
			{
				// Foreign key under the verifier's own kid: proves the signature is
				// checked, not merely that a key was found by kid.
				name:      "foreign key, same kid",
				token:     signTestToken(t, newTestKey(t), kid, testIssuer, xtime.UTCNow().Add(time.Hour), []entity.Role{entity.RoleGuest}),
				targetErr: apperr.ErrInvalidAccessToken,
			},
			{
				// Own key under a kid the storage does not hold: proves lookup is by
				// kid, not "any key will do".
				name:      "own key, foreign kid",
				token:     signTestToken(t, key, "kid-foreign", testIssuer, xtime.UTCNow().Add(time.Hour), []entity.Role{entity.RoleGuest}),
				targetErr: apperr.ErrInvalidAccessToken,
			},
			{
				name:      "alg none",
				token:     signUnsignedToken(t, kid, testIssuer, xtime.UTCNow().Add(time.Hour)),
				targetErr: apperr.ErrInvalidAccessToken,
			},
			{
				name:      "hs256",
				token:     signHS256Token(t, kid, testIssuer, xtime.UTCNow().Add(time.Hour)),
				targetErr: apperr.ErrInvalidAccessToken,
			},
			{
				name:      "malformed",
				token:     "not-a-jwt",
				targetErr: apperr.ErrInvalidAccessToken,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := verifier.VerifyAccessToken(ctx, tt.token)
				require.Error(t, err)
				require.ErrorIs(t, err, tt.targetErr)
			})
		}
	})

	// Deliberate library behavior, not a regression: with no kid header keyfunc
	// hands back the whole key set instead of matching, so the token verifies. The
	// signature is still checked — the old HTTP path behaved identically.
	t.Run("kid-less token verifies", func(t *testing.T) {
		t.Parallel()

		key := newTestKey(t)
		// A kid unrelated to the other cases: the token carries no kid at all, so
		// whatever the verifier is configured with must be irrelevant here.
		verifier := newTestVerifier(ctx, t, key, "kid-unrelated")

		token := jwt.NewWithClaims(jwt.SigningMethodES256,
			testClaims(testIssuer, time.Now().Add(time.Hour), []entity.Role{entity.RoleGuest}))
		signed, err := token.SignedString(key)
		require.NoError(t, err)

		claims, err := verifier.VerifyAccessToken(ctx, signed)
		require.NoError(t, err)
		require.Equal(t, []entity.Role{entity.RoleGuest}, claims.UserRoles)

		// ...and the signature is genuinely verified even without a kid.
		foreign := jwt.NewWithClaims(jwt.SigningMethodES256,
			testClaims(testIssuer, time.Now().Add(time.Hour), []entity.Role{entity.RoleGuest}))
		foreignSigned, err := foreign.SignedString(newTestKey(t))
		require.NoError(t, err)

		_, err = verifier.VerifyAccessToken(ctx, foreignSigned)
		require.ErrorIs(t, err, apperr.ErrInvalidAccessToken)
	})
}

// TestVerifyAccessToken_AuthUnavailableUnreachable pins the structure of the
// ErrAuthUnavailable gate rather than just its output. An unknown kid is the only
// input that reaches verify_access_token.go's ErrKeyNotFound branch, so it is the
// only token that can exercise the second conjunct. Both of that conjunct's inputs
// are asserted directly: the local storage always holds the one key, and the
// refresh callback is wired nowhere, so the timestamp stays zero. The test fails if
// someone reattaches a refresh callback to the local path.
func TestVerifyAccessToken_AuthUnavailableUnreachable(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	key := newTestKey(t)
	verifier := newTestVerifier(ctx, t, key, "kid-1")

	token := signTestToken(t, key, "kid-unknown", testIssuer, time.Now().Add(time.Hour), []entity.Role{entity.RoleGuest})

	_, err := verifier.VerifyAccessToken(ctx, token)
	require.ErrorIs(t, err, apperr.ErrInvalidAccessToken)
	require.NotErrorIs(t, err, apperr.ErrAuthUnavailable)

	keys, err := verifier.keyfunc.Storage().KeyReadAll(ctx)
	require.NoError(t, err)
	require.Len(t, keys, 1, "local storage must hold exactly the configured key")
	require.Zero(t, verifier.LastRefreshFailedAt(ctx), "refresh callback must not be wired to the local key path")
}

func TestVerifyAccessToken_Concurrent(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	const goroutines = 16

	key := newTestKey(t)
	kid := "kid-1"
	verifier := newTestVerifier(ctx, t, key, kid)

	valid := signTestToken(t, key, kid, testIssuer, time.Now().Add(time.Hour), []entity.Role{entity.RoleEditor})
	unknownKid := signTestToken(t, key, "kid-unknown", testIssuer, time.Now().Add(time.Hour), []entity.Role{entity.RoleEditor})

	// Results are carried back and asserted on the test goroutine: require calls
	// FailNow, which is only legal there — from a spawned goroutine it would
	// Goexit past the WaitGroup instead of failing the test.
	type result struct {
		roles []entity.Role
		err   error
	}
	results := make([]result, goroutines)

	var wg sync.WaitGroup
	for i := range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()

			token := unknownKid
			if i%2 == 0 {
				token = valid
			}

			claims, err := verifier.VerifyAccessToken(ctx, token)
			results[i] = result{err: err}
			if err == nil {
				results[i].roles = claims.UserRoles
			}
		}()
	}
	wg.Wait()

	for i, got := range results {
		if i%2 == 0 {
			require.NoErrorf(t, got.err, "goroutine %d verifying the valid token", i)
			require.Equalf(t, []entity.Role{entity.RoleEditor}, got.roles, "goroutine %d", i)
			continue
		}

		require.ErrorIsf(t, got.err, apperr.ErrInvalidAccessToken, "goroutine %d verifying the unknown kid", i)
	}

	require.Zero(t, verifier.LastRefreshFailedAt(ctx))
}

func signUnsignedToken(t *testing.T, kid, issuer string, expiresAt time.Time) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodNone, testClaims(issuer, expiresAt, []entity.Role{entity.RoleGuest}))
	token.Header["kid"] = kid

	signed, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	return signed
}

func signHS256Token(t *testing.T, kid, issuer string, expiresAt time.Time) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, testClaims(issuer, expiresAt, []entity.Role{entity.RoleGuest}))
	token.Header["kid"] = kid

	signed, err := token.SignedString([]byte("shared-secret"))
	require.NoError(t, err)

	return signed
}

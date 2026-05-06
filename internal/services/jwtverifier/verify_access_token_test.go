package jwtverifier

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

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
		server := newJWKSServer(t, &jwkState{key: key, kid: kid})

		verifier := newTestVerifier(ctx, t, server.URL)
		token := signTestToken(t, key, kid, testIssuer, time.Now().Add(time.Hour), []entity.Role{entity.RoleEditor})

		claims, err := verifier.VerifyAccessToken(ctx, token)
		require.NoError(t, err)
		require.Equal(t, "alice@example.com", claims.Email)
		require.Equal(t, []entity.Role{entity.RoleEditor}, claims.Roles)
		require.NotEmpty(t, claims.Subject)
	})

	t.Run("refreshes unknown kid", func(t *testing.T) {
		t.Parallel()

		key1 := newTestKey(t)
		state := &jwkState{key: key1, kid: "kid-1"}
		server := newJWKSServer(t, state)

		verifier := newTestVerifier(ctx, t, server.URL)

		key2 := newTestKey(t)
		kid2 := "kid-2"
		state.key = key2
		state.kid = kid2

		token := signTestToken(t, key2, kid2, testIssuer, time.Now().Add(time.Hour), []entity.Role{entity.RoleReviewer})

		claims, err := verifier.VerifyAccessToken(ctx, token)
		require.NoError(t, err)
		require.Equal(t, []entity.Role{entity.RoleReviewer}, claims.Roles)
		require.GreaterOrEqual(t, state.requests.Load(), int64(2))
	})

	t.Run("errors", func(t *testing.T) {
		t.Parallel()

		key := newTestKey(t)
		kid := "kid-1"
		server := newJWKSServer(t, &jwkState{key: key, kid: kid})
		verifier := newTestVerifier(ctx, t, server.URL)

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
				name:      "bad signature",
				token:     signTestToken(t, newTestKey(t), kid, testIssuer, xtime.UTCNow().Add(time.Hour), []entity.Role{entity.RoleGuest}),
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
}

func TestVerifyAccessToken_JWKSUnavailable(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	t.Run("without cached key", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		t.Cleanup(server.Close)

		verifier := newTestVerifier(ctx, t, server.URL)
		token := signTestToken(t, newTestKey(t), "kid-1", testIssuer, time.Now().Add(time.Hour), []entity.Role{entity.RoleGuest})

		_, err := verifier.VerifyAccessToken(ctx, token)
		require.ErrorIs(t, err, apperr.ErrAuthUnavailable)
	})

	t.Run("without matching cached key", func(t *testing.T) {
		t.Parallel()

		key1 := newTestKey(t)
		key2 := newTestKey(t)
		var unavailable atomic.Bool
		state := &jwkState{key: key1, kid: "kid-1"}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			state.requests.Add(1)
			if unavailable.Load() {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"keys": []map[string]any{jwkFromKey(state.key, state.kid)},
			}))
		}))
		t.Cleanup(server.Close)

		verifier := newTestVerifier(ctx, t, server.URL)
		unavailable.Store(true)

		token := signTestToken(t, key2, "kid-2", testIssuer, time.Now().Add(time.Hour), []entity.Role{entity.RoleGuest})

		_, err := verifier.VerifyAccessToken(ctx, token)
		require.ErrorIs(t, err, apperr.ErrAuthUnavailable)
	})
}

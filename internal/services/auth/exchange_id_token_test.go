package auth

import (
	"context"
	"testing"
	"time"

	"github.com/ruko1202/xlog"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap/zaptest"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

func TestExchangeIDToken(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	t.Run("happy path - new user", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initService(t)

		expClaims := &entity.OAuthIDTokenClaims{
			Subject: xuuid.NewString(),
			Email:   xuuid.NewString() + "@example.com",
			Name:    "Alice",
		}
		mocks.oauthProvider.EXPECT().
			VerifyToken(gomock.Any(), "google-id-token").
			Return(expClaims, nil)

		pair, err := srv.ExchangeIDToken(ctx, &entity.ExchangeIDTokenCmd{
			Provider: entity.OAuthProviderGoogle,
			IDToken:  "google-id-token",
			ClientIP: "10.0.0.1",
		})
		require.NoError(t, err)
		require.NotNil(t, pair)
		require.NotEmpty(t, pair.AccessToken)
		require.NotEmpty(t, pair.RefreshToken)
		require.Equal(t, int(cfg.JWT.AccessTokenTTL.Seconds()), pair.ExpiresIn)

		claims, err := srv.tokenSrv.VerifyAccessToken(ctx, pair.AccessToken)
		require.NoError(t, err)
		require.NotEmpty(t, claims.Subject)
		require.NotEqualValues(t, expClaims.Subject, claims.Subject)
		require.Equal(t, expClaims.Email, claims.Email)
		require.True(t, claims.IssuedAt.After(xtime.UTCNow().Add(-time.Minute)))
	})

	t.Run("idempotent - same sub maps to same user", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initService(t)

		expClaims := &entity.OAuthIDTokenClaims{
			Subject: xuuid.NewString(),
			Email:   xuuid.NewString() + "@example.com",
			Name:    "Bob",
		}
		mocks.oauthProvider.EXPECT().
			VerifyToken(gomock.Any(), gomock.Any()).
			Return(expClaims, nil).
			Times(2)

		pair1, err := srv.ExchangeIDToken(ctx, &entity.ExchangeIDTokenCmd{
			Provider: entity.OAuthProviderGoogle,
			IDToken:  "tok-1",
			ClientIP: "10.0.0.1",
		})
		require.NoError(t, err)

		pair2, err := srv.ExchangeIDToken(ctx, &entity.ExchangeIDTokenCmd{
			Provider: entity.OAuthProviderGoogle,
			IDToken:  "tok-2",
			ClientIP: "10.0.0.1",
		})
		require.NoError(t, err)

		claims1, err := srv.tokenSrv.VerifyAccessToken(ctx, pair1.AccessToken)
		require.NoError(t, err)
		claims2, err := srv.tokenSrv.VerifyAccessToken(ctx, pair2.AccessToken)
		require.NoError(t, err)
		require.Equal(t, claims1.Subject, claims2.Subject)
	})

	t.Run("invalid id token", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initService(t)

		mocks.oauthProvider.EXPECT().
			VerifyToken(gomock.Any(), gomock.Any()).
			Return(nil, apperr.ErrInvalidAccessToken)

		pair, err := srv.ExchangeIDToken(ctx, &entity.ExchangeIDTokenCmd{
			Provider: entity.OAuthProviderGoogle,
			IDToken:  "bad",
			ClientIP: "10.0.0.1",
		})
		require.Nil(t, pair)
		require.ErrorIs(t, err, apperr.ErrInvalidAccessToken)
	})

	t.Run("unsupported provider", func(t *testing.T) {
		t.Parallel()

		srv, _ := initService(t)
		pair, err := srv.ExchangeIDToken(ctx, &entity.ExchangeIDTokenCmd{
			Provider: entity.OAuthProviderGithub,
			IDToken:  "tok",
			ClientIP: "10.0.0.1",
		})
		require.Nil(t, pair)
		require.ErrorIs(t, err, apperr.ErrUnsupportedProvider)
	})
}

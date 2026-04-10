package auth

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ruko1202/xlog"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap/zaptest"

	"github.com/ruko1202/maintmode/internal/entity"

	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

func TestHandleCallback(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	t.Run("new user", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initService(t)

		oauthUser := handleCallbackMock(mocks, 1)

		pair, err := srv.HandleOAuthCallback(ctx, &entity.HandleOAuthCallbackCmd{
			Provider:     entity.OAuthProviderGoogle,
			CallbackCode: "code-1",
			ClientIP:     "10.0.0.1",
		})
		require.NoError(t, err)
		require.NotNil(t, pair)
		require.NotEmpty(t, pair.AccessToken)
		require.NotEmpty(t, pair.RefreshToken)
		require.Equal(t, int(accessTokenTTL.Seconds()), pair.ExpiresIn)

		// Verify access token claims
		claims, err := srv.tokenSrv.VerifyAccessToken(ctx, pair.AccessToken)
		require.NoError(t, err)
		require.NotNil(t, claims)
		require.NotEmpty(t, claims.ID)
		require.NotEmpty(t, claims.Subject)
		require.Equal(t, tokenIssuer, claims.Issuer)
		require.True(t, claims.IssuedAt.After(xtime.UTCNow().Add(-time.Minute)))
		require.True(t, claims.ExpiresAt.After(xtime.UTCNow().Add(accessTokenTTL-time.Minute)))
		require.Equal(t, oauthUser.Email, claims.Email)
		require.Equal(t, entity.DefaultRoles, claims.Roles)
	})

	t.Run("existingUser", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initService(t)

		handleCallbackMock(mocks, 2)

		// First callback creates user
		pair1, err := srv.HandleOAuthCallback(ctx, &entity.HandleOAuthCallbackCmd{
			Provider:     entity.OAuthProviderGoogle,
			CallbackCode: "code-1",
			ClientIP:     "10.0.0.1",
		})
		require.NoError(t, err)
		require.NotNil(t, pair1)

		// Second callback should find existing user
		pair2, err := srv.HandleOAuthCallback(ctx, &entity.HandleOAuthCallbackCmd{
			Provider:     entity.OAuthProviderGoogle,
			CallbackCode: "code-1",
			ClientIP:     "10.0.0.1",
		})
		require.NoError(t, err)
		require.NotNil(t, pair2)

		require.NotEqualValues(t, pair1.AccessToken, pair2.AccessToken)
	})

	t.Run("invalid code", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initService(t)

		mocks.oauthProvider.EXPECT().
			Exchange(gomock.Any(), gomock.Any()).
			Return(nil, fmt.Errorf("invalid code"))

		pair, err := srv.HandleOAuthCallback(ctx, &entity.HandleOAuthCallbackCmd{
			Provider:     entity.OAuthProviderGoogle,
			CallbackCode: "code-1",
			ClientIP:     "10.0.0.1",
		})
		require.Nil(t, pair)
		require.Error(t, err)
	})

	t.Run("invalid code", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initService(t)

		mocks.oauthProvider.EXPECT().
			Exchange(gomock.Any(), gomock.Any()).
			Return(nil, fmt.Errorf("invalid code"))

		pair, err := srv.HandleOAuthCallback(ctx, &entity.HandleOAuthCallbackCmd{
			Provider:     entity.OAuthProviderGoogle,
			CallbackCode: "code-1",
			ClientIP:     "10.0.0.1",
		})
		require.Nil(t, pair)
		require.Error(t, err)
	})
}

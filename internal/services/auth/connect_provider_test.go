package auth

import (
	"context"
	"testing"

	"github.com/ruko1202/xlog"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap/zaptest"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

func TestConnectProvider(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	t.Run("ok - verifies token and links identity", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initService(t)

		// Seed a user whose only identity is github, so connecting google adds a
		// new provider (one identity per provider).
		user, err := srv.usersSrv.GetOrCreateByOAuthInfo(ctx, entity.OAuthProviderGithub, &entity.OAuthProviderUserInfo{
			ID:    xuuid.NewString(),
			Email: xuuid.NewString() + "@example.com",
			Name:  "User",
		})
		require.NoError(t, err)

		// Connect google (the mock provider resolves to google).
		connectClaims := &entity.OAuthIDTokenClaims{
			Subject: xuuid.NewString(),
			Email:   xuuid.NewString() + "@example.com",
			Name:    "User",
		}
		mocks.oauthProvider.EXPECT().
			VerifyToken(gomock.Any(), "id-token").
			Return(connectClaims, nil)

		err = srv.ConnectProvider(ctx, &entity.ConnectProviderCmd{
			UserID:   user.ID,
			Provider: entity.OAuthProviderGoogle,
			IDToken:  "id-token",
		})
		require.NoError(t, err)
	})

	t.Run("invalid id token", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initService(t)

		mocks.oauthProvider.EXPECT().
			VerifyToken(gomock.Any(), gomock.Any()).
			Return(nil, apperr.ErrInvalidAccessToken)

		err := srv.ConnectProvider(ctx, &entity.ConnectProviderCmd{
			UserID:   xuuid.New(),
			Provider: entity.OAuthProviderGoogle,
			IDToken:  "bad",
		})
		require.ErrorIs(t, err, apperr.ErrInvalidAccessToken)
	})

	t.Run("unsupported provider", func(t *testing.T) {
		t.Parallel()

		srv, _ := initService(t)

		err := srv.ConnectProvider(ctx, &entity.ConnectProviderCmd{
			UserID:   xuuid.New(),
			Provider: entity.OAuthProviderGithub,
			IDToken:  "tok",
		})
		require.ErrorIs(t, err, apperr.ErrUnsupportedProvider)
	})
}

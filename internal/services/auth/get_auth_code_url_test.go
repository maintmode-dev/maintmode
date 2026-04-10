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
)

func TestAuthCodeURL(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initService(t)

		mocks.oauthProvider.EXPECT().
			AuthCodeURL(gomock.Any(), gomock.Any()).
			Return("my-url")

		url, err := srv.GetAuthCodeURL(ctx, &entity.GetAuthCodeURLCmd{
			Provider: entity.OAuthProviderGoogle,
			State:    &entity.OAuthState{Nonce: "my-nonce"},
		})
		require.NoError(t, err)
		require.Equal(t, "my-url", url)
	})

	t.Run("unsupported provider", func(t *testing.T) {
		t.Parallel()

		srv, _ := initService(t)

		url, err := srv.GetAuthCodeURL(ctx, &entity.GetAuthCodeURLCmd{
			Provider: "unsupported",
			State:    &entity.OAuthState{Nonce: "my-nonce"},
		})
		require.ErrorIs(t, err, apperr.ErrUnsupportedProvider)
		require.Empty(t, url)
	})
}

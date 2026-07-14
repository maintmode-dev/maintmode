package messagesender

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	mock_notifytransport "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/services/notifytransport"
)

// senderWith builds a Service whose resolver yields (tr, err) once.
func senderWith(t *testing.T, tr *mock_notifytransport.MockTransport, resolveErr error) *Service {
	t.Helper()
	resolver := mock_notifytransport.NewMockTransportResolver(gomock.NewController(t))
	if tr == nil {
		resolver.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, resolveErr)
	} else {
		resolver.EXPECT().Get(gomock.Any(), gomock.Any()).Return(tr, resolveErr)
	}
	return NewService(resolver, nil)
}

// Send resolves the transport and delivers, propagating both classes of error
// (resolve failure carries the integration sentinel; a send failure is the
// transport's own error). A disabled integration keeps ErrIntegrationDisabled
// so the caller can treat it as the routine best-effort drop.
func TestSend(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	msg := entity.NotifyMessage{Subject: "s", Body: "b"}

	t.Run("delivered", func(t *testing.T) {
		t.Parallel()
		tr := mock_notifytransport.NewMockTransport(gomock.NewController(t))
		tr.EXPECT().Send(gomock.Any(), "C1", gomock.Any()).Return(nil)

		require.NoError(t, senderWith(t, tr, nil).Send(ctx, entity.NotifyTransportSlack, "C1", msg))
	})

	t.Run("disabled keeps its sentinel", func(t *testing.T) {
		t.Parallel()
		err := senderWith(t, nil, apperr.ErrIntegrationDisabled).Send(ctx, entity.NotifyTransportSlack, "C1", msg)
		require.ErrorIs(t, err, apperr.ErrIntegrationDisabled)
	})

	t.Run("resolve error propagates", func(t *testing.T) {
		t.Parallel()
		err := senderWith(t, nil, errors.New("unwrap dek: unknown kek")).Send(ctx, entity.NotifyTransportSlack, "C1", msg)
		require.Error(t, err)
	})

	t.Run("send error propagates", func(t *testing.T) {
		t.Parallel()
		tr := mock_notifytransport.NewMockTransport(gomock.NewController(t))
		sendErr := errors.New("401 revoked")
		tr.EXPECT().Send(gomock.Any(), "C1", gomock.Any()).Return(sendErr)

		require.ErrorIs(t, senderWith(t, tr, nil).Send(ctx, entity.NotifyTransportSlack, "C1", msg), sendErr)
	})
}

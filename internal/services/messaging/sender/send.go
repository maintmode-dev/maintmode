package messagesender

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

// Send delivers msg synchronously. Do NOT call from inside a DB tx —
// the blocking network call would hold row locks
func (s *Service) Send(ctx context.Context, trName entity.NotifyTransport, target string, msg entity.NotifyMessage) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Messaging.Send",
		xfield.String("transport", string(trName)),
		xfield.String("target", target),
	)
	defer span.End()

	if _, ok := dbtx.TxFromContext(ctx); ok {
		return fmt.Errorf("not allowed calling inside DB tx. use SendAsync instead")
	}

	tr, err := s.notifyTransportRegistry.Get(ctx, trName)
	if err != nil {
		xlog.Error(ctx, "get transport", xfield.Error(err))
		return fmt.Errorf("no transport %q: %w", trName, err)
	}

	err = tr.Send(ctx, target, msg)
	if err != nil {
		xlog.Error(ctx, "messaging send failed", xfield.Error(err))
		return err
	}

	return nil
}

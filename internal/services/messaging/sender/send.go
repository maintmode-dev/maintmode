package messagesender

import (
	"context"
	"errors"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

// Send delivers msg synchronously, threading it under replyTo when one is
// given. Do NOT call from inside a DB tx — the blocking network call would hold
// row locks.
func (s *Service) Send(
	ctx context.Context,
	trName entity.NotifyTransport,
	target string,
	msg entity.NotifyMessage,
	replyTo *entity.MessageRef,
) (entity.SendResult, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Messaging.Send",
		xfield.String("transport", string(trName)),
		xfield.String("target", target),
	)
	defer span.End()

	if _, ok := dbtx.TxFromContext(ctx); ok {
		return entity.SendResult{}, fmt.Errorf("not allowed calling inside DB tx. use SendAsync instead")
	}

	tr, err := s.notifyTransportRegistry.Get(ctx, trName)
	if err != nil {
		// A disabled/unconfigured integration (RUK-196) is a routine drop, not a
		// fault — log it at Warn so it doesn't page on the Error channel, matching
		// the async processor. The error still propagates; the caller decides.
		if errors.Is(err, apperr.ErrIntegrationDisabled) {
			xlog.Warn(ctx, "messaging send: integration disabled, dropping delivery",
				xfield.String("transport", string(trName)))
			return entity.SendResult{}, fmt.Errorf("no transport %q: %w", trName, err)
		}
		xlog.Error(ctx, "get transport", xfield.Error(err))
		return entity.SendResult{}, fmt.Errorf("no transport %q: %w", trName, err)
	}

	res, err := tr.Send(ctx, target, msg, replyTo)
	if err != nil {
		xlog.Error(ctx, "messaging send failed", xfield.Error(err))
		return entity.SendResult{}, err
	}

	return res, nil
}

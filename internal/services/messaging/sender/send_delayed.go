package sender

import (
	"context"
	"time"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

// SendDelayed is SendAsync with NextAttemptAt set to now+delay
func (s *Service) SendDelayed(
	ctx context.Context,
	trName entity.MessengerID,
	target string,
	msg entity.Message,
	delay time.Duration,
	opts ...EnqueueOption,
) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Messaging.SendDelayed",
		xfield.String("transport", string(trName)),
		xfield.String("target", target),
	)
	defer span.End()

	return s.enqueue(ctx, trName, target, msg, append(opts, WithDelay(delay))...)
}

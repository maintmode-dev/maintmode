package integration

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/audit"
)

// publishAudit enqueues an audited action to the durable outbox after the
// mutation's tx has committed. A failed enqueue is logged, not propagated: an
// integration change must not fail because the audit publish hiccuped (mirrors
// the maint/auth policy).
func (s *Service) publishAudit(ctx context.Context, action audit.Action) {
	if err := s.auditPublisher.Publish(ctx, action); err != nil {
		xlog.Error(ctx, "failed to publish integration audit action",
			xfield.String("action", fmt.Sprintf("%T", action)),
			xfield.Error(err),
		)
	}
}

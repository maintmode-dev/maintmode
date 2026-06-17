package maint

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/audit"
)

// publishAudit publishes an audited action to the durable outbox. A failed
// enqueue is logged, not propagated: a maintenance mutation must not fail
// because the audit publish hiccuped (mirrors the auth service policy). Called
// after the mutation's tx commits.
func (s *Service) publishAudit(ctx context.Context, action audit.Action) {
	if err := s.auditPublisher.Publish(ctx, action); err != nil {
		xlog.Error(ctx, "failed to publish maint audit action",
			xfield.String("action", fmt.Sprintf("%T", action)),
			xfield.Error(err),
		)
	}
}

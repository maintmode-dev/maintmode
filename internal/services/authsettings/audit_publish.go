package authsettings

import (
	"context"

	"github.com/ruko1202/maintmode/internal/audit"
)

// publishAudit enqueues an audited action, or reports why it could not.
//
// A thin wrapper so the publisher's nil-ness is answered once: a binary wired
// without an audit publisher still toggles methods, it just cannot record them,
// and that is a deployment shape rather than a request failure.
func (s *Service) publishAudit(ctx context.Context, action audit.Action) error {
	if s.auditPublisher == nil {
		return nil
	}

	return s.auditPublisher.Publish(ctx, action)
}

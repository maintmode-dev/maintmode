package auth

import (
	"errors"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// provisioningFailureReason maps a get-or-create failure on a login path to
// its audit reason: a policy refusal (signup disabled, no invitation) is a
// distinct security signal, everything else stays a generic provisioning
// failure.
func provisioningFailureReason(err error) entity.AuditFailureReason {
	if errors.Is(err, apperr.ErrSignupDisabled) {
		return entity.AuditFailureSignupDisabled
	}
	return entity.AuditFailureUserProvisioning
}

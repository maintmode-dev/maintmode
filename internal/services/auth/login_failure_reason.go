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

// issuanceFailureReason is the same distinction one step later.
//
// A blocked user does not fail provisioning — the account resolves fine — it
// fails at IssueTokenPair, which is where ErrUserBlocked is raised. Filing that
// under AuditFailureTokenIssuance would put two unrelated events under one
// reason: "this deployment cannot mint tokens", which is an incident, and "this
// person is blocked", which is the system working as configured. An operator
// reading the trail has to be able to tell those apart, and apperr's own
// doctrine for ErrInvalidCredentials says exactly that about refused accounts.
func issuanceFailureReason(err error) entity.AuditFailureReason {
	if errors.Is(err, apperr.ErrUserBlocked) {
		return entity.AuditFailureUserBlocked
	}
	return entity.AuditFailureTokenIssuance
}

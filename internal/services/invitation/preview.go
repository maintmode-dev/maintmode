package invitation

import (
	"context"
	"errors"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/utils/xtime"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xhash"
)

// Preview validates a raw token from the email link and returns only its
// privacy-safe status (valid|invalid|expired|accepted|revoked). It deliberately
// exposes nothing else: not the email, roles, inviter, or message.
//
// A token that maps to no invitation (or an empty token) reports "invalid"
// rather than 404, so a link-holder cannot distinguish a malformed token from a
// non-existent one.
func (s *Service) Preview(ctx context.Context, rawToken string) (entity.InvitationPreviewStatus, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Invitation.Preview")
	defer span.End()

	// Privacy: an empty or unknown token reports "invalid" with no error, so the
	// endpoint answers 200 either way and a link-holder cannot tell a malformed
	// token from a non-existent one. Only genuine failures propagate.
	if rawToken == "" {
		return entity.InvitationPreviewStatusInvalid, nil
	}

	inv, err := s.store.GetByTokenHash(ctx, xhash.HashSha256([]byte(rawToken)))
	if err != nil {
		if errors.Is(err, apperr.ErrInvitationNotFound) {
			return entity.InvitationPreviewStatusInvalid, nil
		}
		xlog.Error(ctx, "preview invitation failed", xfield.Error(err))
		return "", err
	}

	return inv.PreviewStatus(xtime.UTCNow()), nil
}

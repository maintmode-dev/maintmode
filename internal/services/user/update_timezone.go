package user

import (
	"context"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// UpdateTimezone sets the user's timezone preference and returns the updated
// user. The requested value is validated first (see entity.CanonicalTimezone):
// nil, "" or whitespace reset to auto-detect (stored as NULL), a valid IANA
// identifier is stored trimmed, anything else returns apperr.ErrInvalidTimezone
// (→ HTTP 400).
//
// This is a self-service preference change; it is intentionally not audited.
func (s *Service) UpdateTimezone(ctx context.Context, userID uuid.UUID, tz *string) (*entity.User, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.User.UpdateTimezone")
	defer span.End()

	name, ok := entity.CanonicalTimezone(tz)
	if !ok {
		xlog.Warn(ctx, "invalid timezone preference")
		return nil, apperr.ErrInvalidTimezone
	}

	// Empty canonical name means "reset to auto-detect" — persist NULL.
	var normalized *string
	if name != "" {
		normalized = &name
	}

	return s.updateWithApply(ctx, userID, func(_ context.Context, user *entity.User) error {
		user.Timezone = normalized
		return nil
	})
}

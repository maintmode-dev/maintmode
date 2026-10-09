package token

import (
	"context"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
)

// revokeSessionAccessTokens makes the live access tokens of sessions that were
// just revoked in the database stop working now, instead of when they expire.
// It runs after the refresh-token revocation succeeded, so a failed database
// write never strands a session that is still alive.
//
// A failure here is logged, not returned. The session itself is already over
// -- no refresh can extend it -- and failing the caller would not help: a
// retry finds nothing left to revoke. What is lost is the early cut-off; the
// tokens fall back to expiring on their TTL, which is how every revocation
// behaved before. This mirrors the per-token blacklist on logout.
func (s *Service) revokeSessionAccessTokens(ctx context.Context, sessionIDs ...uuid.UUID) {
	if len(sessionIDs) == 0 {
		return
	}

	if err := s.revokedSessions.AddSessions(ctx, s.revokedSessionTTL, sessionIDs...); err != nil {
		xlog.Error(ctx, "failed to revoke the access tokens of revoked sessions; they stay valid until they expire",
			xfield.Int("sessions", len(sessionIDs)),
			xfield.Error(err),
		)
	}
}

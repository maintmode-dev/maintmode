package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// EnsureActiveToken reports the token subject's CURRENT roles when the access
// token is active, and an error otherwise — the active-token gate the API
// middleware applies to every mutation (reads get EnsureNotRevoked). "Active"
// is the full check (JWT + blacklist + blocked-user): an inactive token yields
// ErrInvalidAccessToken, while a transient store failure propagates so the
// middleware fails closed.
//
// The roles come from the user store, not from the token claims: a role revoked
// after the token was minted must stop authorizing writes now, not when the
// token expires.
func (s *Service) EnsureActiveToken(ctx context.Context, tokenString string) ([]entity.Role, error) {
	report, err := s.Introspect(ctx, tokenString)
	if err != nil {
		return nil, err
	}
	if !report.Active {
		return nil, apperr.ErrInvalidAccessToken
	}
	return report.Roles, nil
}

// EnsureNotRevoked is the active-token gate the API middleware applies to every
// read: it refuses a token that was revoked on its own (logout) or through its
// session, and nothing more. It deliberately skips the user-store lookup of
// EnsureActiveToken, so a read costs one Valkey EXISTS and no Postgres round
// trip; a blocked user is still cut off, because a block revokes their
// sessions. An inactive token yields ErrInvalidAccessToken; a store failure
// propagates so the middleware fails closed.
func (s *Service) EnsureNotRevoked(ctx context.Context, tokenString string) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.EnsureNotRevoked")
	defer span.End()

	claims, err := s.tokenSrv.VerifyAccessToken(ctx, tokenString)
	if err != nil {
		return err
	}
	if claims.ID == "" {
		xlog.Warn(ctx, "empty access token ID")
		return apperr.ErrInvalidAccessToken
	}

	revoked, err := s.isRevoked(ctx, claims)
	if err != nil {
		return err
	}
	if revoked {
		return apperr.ErrInvalidAccessToken
	}
	return nil
}

// Introspect checks if an access token is active: not blacklisted, its session
// not revoked, its subject an active user.
// Used by downstream services for critical operations (RFC 7662).
func (s *Service) Introspect(ctx context.Context, tokenString string) (*entity.IntrospectReport, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.Introspect")
	defer span.End()

	claims, err := s.tokenSrv.VerifyAccessToken(ctx, tokenString)
	if err != nil {
		xlog.Error(ctx, "failed to verify access token", xfield.Error(err))
		return &entity.IntrospectReport{Active: false}, nil
	}

	if claims.ID == "" {
		xlog.Warn(ctx, "empty access token ID")
		return &entity.IntrospectReport{Active: false}, nil
	}

	revoked, err := s.isRevoked(ctx, claims)
	if err != nil {
		return nil, err
	}
	if revoked {
		return &entity.IntrospectReport{
			Active: false,
			JTI:    claims.ID,
		}, nil
	}

	// A blocked user's live access tokens must stop working on the next
	// introspected (critical-mutation) request, not only after they expire.
	// Issuance is already barred (token.IssueAccessToken); this closes the
	// window for tokens minted before the block.
	user, err := s.activeSubject(ctx, claims.Subject)
	if err != nil {
		// Transient user-store error: fail closed via the error (the gateway
		// rejects with 503) rather than silently treating the token as active.
		return nil, err
	}
	if user == nil {
		xlog.Warn(ctx, "access token subject is not an active user", xfield.String("subject", claims.Subject))
		return &entity.IntrospectReport{
			Active: false,
			JTI:    claims.ID,
		}, nil
	}

	return &entity.IntrospectReport{
		Active:  true,
		JTI:     claims.ID,
		Subject: claims.Subject,
		Email:   claims.UserEmail,
		// Stored roles, not claims: see EnsureActiveToken.
		Roles: user.Roles,
		Exp:   claims.ExpiresAt.Unix(),
	}, nil
}

// isRevoked is one lookup for both ways a token is revoked: on its own (logout
// blacklists the jti) and through its session (any revocation of the
// refresh-token family marks the sid). A store failure is returned so the
// caller fails closed.
func (s *Service) isRevoked(ctx context.Context, claims *entity.AccessClaims) (bool, error) {
	revoked, err := s.blacklistStore.IsRevoked(ctx, claims.ID, claims.SessionID)
	if err != nil {
		xlog.Error(ctx, "failed to check blacklistStore", xfield.Error(err))
		return false, fmt.Errorf("check blacklistStore: %w", err)
	}
	if revoked {
		xlog.Warn(ctx, "access token or its session is revoked", xfield.String("sid", claims.SessionID))
	}
	return revoked, nil
}

// activeSubject returns the real, non-blocked user the JWT subject maps to, or
// nil when there is none. It fails closed: a subject we issued is always a valid
// user UUID and always resolves, so a malformed subject or a missing user means
// the token is not trustworthy (broken issuance, deleted account, or a token we
// shouldn't honor) — treat it as inactive rather than waving it through. Only a
// transient store error propagates, so the caller can reject with 503 instead
// of guessing.
func (s *Service) activeSubject(ctx context.Context, subject string) (*entity.User, error) {
	userID, err := uuid.Parse(subject)
	if err != nil {
		xlog.Warn(ctx, "access token subject is not a valid uuid", xfield.String("subject", subject))
		// Fail closed: a malformed subject is not a transient failure to surface,
		// it means the token is not trustworthy — report inactive, not an error.
		return nil, nil //nolint:nilerr,nilnil // intentional fail-closed, see func doc
	}

	user, err := s.usersSrv.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, apperr.ErrUserNotFound) {
			return nil, nil //nolint:nilnil // a missing subject is inactive, not an error
		}
		xlog.Error(ctx, "failed to load user for introspect block check", xfield.Error(err))
		return nil, fmt.Errorf("load user: %w", err)
	}
	if user.IsBlocked() {
		return nil, nil //nolint:nilnil // a blocked subject is inactive, not an error
	}

	return user, nil
}

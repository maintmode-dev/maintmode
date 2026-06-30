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

// EnsureActiveToken reports nil when the access token is active and an error
// otherwise — the active-token gate the API middleware applies to critical
// mutations. "Active" is the full check (JWT + blacklist + blocked-user): an
// inactive token yields ErrInvalidAccessToken, while a transient store failure
// propagates so the middleware fails closed. It is the in-process replacement
// for the S2S introspect the middleware used to call; the caller only needs the
// yes/no answer, so no claims are returned.
func (s *Service) EnsureActiveToken(ctx context.Context, tokenString string) error {
	report, err := s.Introspect(ctx, tokenString)
	if err != nil {
		return err
	}
	if !report.Active {
		return apperr.ErrInvalidAccessToken
	}
	return nil
}

// Introspect checks if an access token is active (not blacklisted).
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

	blacklisted, err := s.blacklistStore.Contains(ctx, claims.ID)
	if err != nil {
		xlog.Error(ctx, "failed to check blacklistStore", xfield.Error(err))
		return nil, fmt.Errorf("check blacklistStore: %w", err)
	}
	if blacklisted {
		xlog.Warn(ctx, "access token is blacklisted")
		return &entity.IntrospectReport{
			Active: false,
			JTI:    claims.ID,
		}, nil
	}

	// A blocked user's live access tokens must stop working on the next
	// introspected (critical-mutation) request, not only after they expire.
	// Issuance is already barred (token.IssueAccessToken); this closes the
	// window for tokens minted before the block.
	if active, err := s.isSubjectActive(ctx, claims.Subject); err != nil {
		// Transient user-store error: fail closed via the error (the gateway
		// rejects with 503) rather than silently treating the token as active.
		return nil, err
	} else if !active {
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
		Roles:   claims.UserRoles,
		Exp:     claims.ExpiresAt.Unix(),
	}, nil
}

// isSubjectActive reports whether the JWT subject maps to a real, non-blocked
// user. It fails closed: a subject we issued is always a valid user UUID and
// always resolves, so a malformed subject or a missing user means the token is
// not trustworthy (broken issuance, deleted account, or a token we shouldn't
// honor) — treat it as inactive rather than waving it through. Only a transient
// store error propagates, so the caller can reject with 503 instead of guessing.
func (s *Service) isSubjectActive(ctx context.Context, subject string) (bool, error) {
	userID, err := uuid.Parse(subject)
	if err != nil {
		xlog.Warn(ctx, "access token subject is not a valid uuid", xfield.String("subject", subject))
		// Fail closed: a malformed subject is not a transient failure to surface,
		// it means the token is not trustworthy — report inactive, not an error.
		return false, nil //nolint:nilerr // intentional fail-closed, see func doc
	}

	user, err := s.usersSrv.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, apperr.ErrUserNotFound) {
			return false, nil
		}
		xlog.Error(ctx, "failed to load user for introspect block check", xfield.Error(err))
		return false, fmt.Errorf("load user: %w", err)
	}

	return !user.IsBlocked(), nil
}

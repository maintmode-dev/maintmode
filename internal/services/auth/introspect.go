package auth

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

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

	return &entity.IntrospectReport{
		Active:  true,
		JTI:     claims.ID,
		Subject: claims.Subject,
		Email:   claims.UserEmail,
		Roles:   claims.UserRoles,
		Exp:     claims.ExpiresAt.Unix(),
	}, nil
}

package auth

import (
	"context"
	"fmt"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/audit"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// Logout revokes a single refresh token and blacklists the current access token.
func (s *Service) Logout(ctx context.Context, tokenPair *entity.TokenPair) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.Logout")
	defer span.End()

	return s.logout(ctx, tokenPair.AccessToken, func(ctx context.Context, claims *entity.AccessClaims) error {
		if tokenPair.RefreshToken == "" {
			return nil
		}
		return s.tokenSrv.RevokeRefreshToken(ctx, tokenPair.RefreshToken, claims)
	})
}

// LogoutAll revokes all refresh tokensStore for the user and blacklists the current access token.
func (s *Service) LogoutAll(ctx context.Context, accessTokenRaw string) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.LogoutAll")
	defer span.End()

	return s.logout(ctx, accessTokenRaw, func(ctx context.Context, claims *entity.AccessClaims) error {
		userID, err := uuid.Parse(claims.Subject)
		if err != nil {
			return fmt.Errorf("parse user id from claims.Subject: %w", err)
		}
		return s.tokenSrv.RevokeRefreshTokenByUserID(ctx, userID)
	})
}

func (s *Service) logout(
	ctx context.Context,
	accessTokenRaw string,
	revokeFunc func(ctx context.Context, accessClaims *entity.AccessClaims) error,
) error {
	accessClaims, err := s.tokenSrv.VerifyAccessToken(ctx, accessTokenRaw)
	if err != nil {
		xlog.Error(ctx, "failed to verify access token", xfield.Error(err))
		return apperr.ErrInvalidAccessToken
	}

	if err := validateAccessClaims(ctx, accessClaims); err != nil {
		xlog.Error(ctx, "access claims are required")
		return apperr.ErrInvalidAccessToken
	}

	if err := revokeFunc(ctx, accessClaims); err != nil {
		xlog.Error(ctx, "failed to revoke refresh token", xfield.Error(err))
		return err
	}

	// Blacklist access token jti so introspection returns active=false immediately.
	// If access token is expired/absent — skip (it's useless anyway).
	err = s.blacklistStore.Add(ctx, accessClaims.ID, s.cfg.AccessTokenTTL)
	if err != nil {
		xlog.Error(ctx, "failed to blacklist access token", xfield.Error(err))
	}

	s.publishAudit(ctx, audit.LogoutSuccess{
		User: &entity.User{
			ID:    uuid.MustParse(accessClaims.Subject),
			Email: accessClaims.UserEmail,
		},
		SessionID: accessClaims.ID,
	})
	return nil
}

func validateAccessClaims(ctx context.Context, accessClaims *entity.AccessClaims) error {
	return validation.ValidateStructWithContext(ctx, accessClaims,
		validation.Field(&accessClaims.ID, validation.Required),
	)
}

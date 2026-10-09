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

// Logout ends the caller's session -- every token of its refresh-token family,
// refresh and access alike -- and blacklists the current access token.
//
// The session is named by whatever the caller presents: the refresh token, the
// sid of the access token, or both. Without a refresh token the sid alone ends
// the session, so a client that lost its refresh token still logs out for real
// rather than retiring one access token. Only an access token minted before the
// sid claim existed, presented alone, ends nothing but itself.
//
// When both are presented and name different sessions -- a stale access token
// beside a newer refresh token -- both sessions end. Each is the caller's own:
// the refresh token must belong to the access token's user, or the whole logout
// is refused before anything is revoked, and the sid comes from the same signed
// access token as the user, so it needs no separate ownership check. Ending one
// session more than intended costs a sign-in; refusing would leave both alive
// behind a client that believes it signed out.
func (s *Service) Logout(ctx context.Context, tokenPair *entity.TokenPair) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.Logout")
	defer span.End()

	return s.logout(ctx, tokenPair.AccessToken, func(ctx context.Context, claims *entity.AccessClaims) error {
		ended := uuid.Nil
		if tokenPair.RefreshToken != "" {
			family, err := s.tokenSrv.RevokeFamilyByRefreshToken(ctx, tokenPair.RefreshToken, claims)
			if err != nil {
				return err
			}
			ended = family
		}

		return s.revokeAccessTokenSession(ctx, claims, ended)
	})
}

// revokeAccessTokenSession ends the session the access token was minted for,
// unless it is the one already ended.
func (s *Service) revokeAccessTokenSession(ctx context.Context, claims *entity.AccessClaims, ended uuid.UUID) error {
	// Minted before access tokens carried their session: there is nothing to
	// name it by, and the jti blacklist is all a logout can do.
	if claims.SessionID == "" {
		return nil
	}

	sessionID, err := uuid.Parse(claims.SessionID)
	if err != nil {
		return fmt.Errorf("%w: parse sid: %w", apperr.ErrInvalidAccessToken, err)
	}

	if sessionID == ended {
		return nil
	}

	return s.tokenSrv.RevokeRefreshTokenByFamily(ctx, sessionID)
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
		SessionID: auditSessionID(accessClaims),
	})
	return nil
}

// auditSessionID names the session a logout was made from, for the audit row:
// the refresh-token family the access token was minted for -- the value
// login.success records -- so the sign-in and the sign-out of one session share
// a session_id.
//
// A token minted before the sid claim existed carries no family. Its jti is the
// only handle it has, and recording something beats recording nothing; such
// tokens are gone within one access TTL of the rollout.
func auditSessionID(claims *entity.AccessClaims) string {
	if claims.SessionID != "" {
		return claims.SessionID
	}

	return claims.ID
}

func validateAccessClaims(ctx context.Context, accessClaims *entity.AccessClaims) error {
	return validation.ValidateStructWithContext(ctx, accessClaims,
		validation.Field(&accessClaims.ID, validation.Required),
	)
}

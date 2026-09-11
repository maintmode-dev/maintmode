package auth

import (
	"context"

	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xcripto"
)

// ChangePassword sets the caller's own password.
//
// The write itself -- new hash and session eviction, together or not at all --
// is installPassword's; this function is the proof that the caller may ask for
// it, and the audit trail afterwards.
//
// KeepFamily names the session to spare. It is optional: when the caller cannot
// supply one, every session goes, including theirs. Requiring it would lock out
// the admin who has lost their refresh token -- the very case the break-glass
// flow exists to rescue.
func (s *Service) ChangePassword(ctx context.Context, cmd *entity.ChangePasswordCmd) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.ChangePassword")
	defer span.End()

	if err := xcripto.ValidatePasswordPolicy(cmd.NewPassword); err != nil {
		// Wrapped into ErrValidation so the mapper answers 400 rather than
		// falling through to its default 500. This endpoint is authenticated and
		// the caller owns the account, so unlike the reset path there is nothing
		// to hide by being vague -- a client should be told the password was too
		// short.
		return fmt.Errorf("%w: %w", apperr.ErrValidation, err)
	}

	// Resolved rather than synthesized: the audit renderer prints the actor's
	// address, and a bare id would leave every password event nameless in the
	// one trail an operator reads after a credential incident.
	user, err := s.usersSrv.GetByID(ctx, cmd.UserID)
	if err != nil {
		return fmt.Errorf("resolve user: %w", err)
	}

	if verifyErr := s.verifyCurrentPassword(ctx, cmd); verifyErr != nil {
		return verifyErr
	}

	err = s.installPassword(ctx, cmd.UserID, cmd.NewPassword,
		func(txCtx context.Context) error {
			return s.revokeOtherSessions(txCtx, cmd)
		})
	if err != nil {
		return err
	}

	s.publishAudit(ctx, audit.PasswordChanged{
		User: user,
		Meta: &entity.AuditMetadata{IP: cmd.ClientIP, UserAgent: cmd.UserAgent},
	})
	return nil
}

// verifyCurrentPassword re-proves possession when the user already has a
// password. A stolen access token must not be enough to rewrite the credential
// it was minted from.
//
// When no password exists -- the state right after a break-glass login -- there
// is nothing to prove and supplying one is a client error rather than a
// silently ignored field.
func (s *Service) verifyCurrentPassword(ctx context.Context, cmd *entity.ChangePasswordCmd) error {
	cred, err := s.passwords.GetPasswordByUserID(ctx, cmd.UserID)
	if err != nil {
		if !errors.Is(err, apperr.ErrAuthCredentialNotFound) {
			return fmt.Errorf("read password credential: %w", err)
		}

		if cmd.CurrentPassword != "" {
			return fmt.Errorf("%w: no password is set for this account", apperr.ErrValidation)
		}

		return nil
	}

	if cmd.CurrentPassword == "" {
		return fmt.Errorf("%w: the current password is required", apperr.ErrValidation)
	}

	matches, err := xcripto.VerifyPassword(cred.SecretHash, cmd.CurrentPassword)
	if err != nil {
		xlog.Error(ctx, "stored password credential is unreadable", xfield.Error(err))
		return apperr.ErrInvalidCredentials
	}

	if !matches {
		return apperr.ErrInvalidCredentials
	}

	return nil
}

// revokeOtherSessions evicts every session but the caller's, or every session
// when the caller named none.
func (s *Service) revokeOtherSessions(ctx context.Context, cmd *entity.ChangePasswordCmd) error {
	if cmd.RefreshToken == "" {
		if err := s.tokenSrv.RevokeRefreshTokenByUserID(ctx, cmd.UserID); err != nil {
			return fmt.Errorf("revoke sessions: %w", err)
		}

		return nil
	}

	family, err := s.tokenSrv.FamilyByRefreshToken(ctx, cmd.RefreshToken, cmd.UserID)
	if err != nil {
		// A refresh token that is not this user's, or already dead, is a client
		// error rather than a license to revoke everything: the caller asked to
		// keep a specific session and that request cannot be honored.
		return apperr.ErrInvalidRefreshToken
	}

	if err = s.tokenSrv.RevokeRefreshTokenByUserIDExceptFamily(ctx, cmd.UserID, family); err != nil {
		return fmt.Errorf("revoke other sessions: %w", err)
	}

	return nil
}

// HasPassword reports whether the user holds a password of their own.
//
// The client needs this to decide which form to draw -- setting a first
// password or replacing an existing one -- and it cannot work it out on its
// own: nothing else in the profile implies it, and a value inferred at sign-in
// would not survive a reload.
func (s *Service) HasPassword(ctx context.Context, userID uuid.UUID) (bool, error) {
	_, err := s.passwords.GetPasswordByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, apperr.ErrAuthCredentialNotFound) {
			return false, nil
		}

		return false, fmt.Errorf("read password credential: %w", err)
	}

	return true, nil
}

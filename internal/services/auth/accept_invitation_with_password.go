package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xcripto"
)

// AcceptInvitationWithPassword accepts an invitation by setting a password: it
// creates the invited user, installs the password, spends the invitation and
// grants its roles, then issues a token pair.
//
// The invitation link is the proof of identity. It was delivered to the
// invited address, so the account takes that address and the caller submits
// none -- the role the provider's verified claim plays on the OAuth paths.
//
// Creating the user, storing the password and claiming the invitation run in
// ONE transaction. ClaimForUser's MarkAccepted is the single-use gate, so a
// concurrent acceptance that loses it rolls back the user and password it
// created instead of leaving an account behind. Issuance follows the commit,
// for the reason the invited dance gives: the access token carries the roles
// the user holds when it is minted.
//
// Refusals, in order:
//   - a password outside the policy: ErrValidation, before the token is read;
//   - a token naming no live invitation: apperr.ErrInvalidInvitation;
//   - email_password switched off: apperr.ErrSignInMethodDisabled, since the
//     account could not sign in with the password it was given;
//   - an address that already has an account: apperr.ErrUserAlreadyExists --
//     a password is never attached to an existing account on the strength of a
//     link.
func (s *Service) AcceptInvitationWithPassword(
	ctx context.Context,
	cmd *entity.AcceptInvitationWithPasswordCmd,
) (*entity.TokenPair, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.AcceptInvitationWithPassword")
	defer span.End()

	if err := xcripto.ValidatePasswordPolicy(cmd.Password); err != nil {
		return nil, fmt.Errorf("%w: %w", apperr.ErrValidation, err)
	}

	inv, err := s.invitations.ResolveByToken(ctx, cmd.Token)
	if err != nil {
		return nil, fmt.Errorf("resolve invitation: %w", err)
	}

	meta := &entity.AuditMetadata{IP: cmd.ClientIP, UserAgent: cmd.UserAgent}
	invitee := &entity.User{Email: inv.Email}

	if !s.methodOffered(ctx, entity.AuthMethodNameEmailPassword, cmd.ClientIP) {
		s.publishLoginFailure(ctx, invitee, meta, entity.AuditFailureMethodDisabled)

		return nil, apperr.ErrSignInMethodDisabled
	}

	// Hashed outside the transaction: argon2id is deliberately slow, and a
	// transaction held open across it would hold its locks just as long.
	hash, err := xcripto.HashPassword(cmd.Password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	var user *entity.User
	err = s.txManager.WithinTx(ctx, func(ctx context.Context) error {
		created, txErr := s.usersSrv.CreateWithoutIdentity(ctx, inv.Email, inv.Email)
		if txErr != nil {
			return fmt.Errorf("create user: %w", txErr)
		}

		if txErr = s.passwords.UpsertPassword(ctx, created.ID, hash); txErr != nil {
			return fmt.Errorf("store password: %w", txErr)
		}

		user, txErr = s.invitations.ClaimForUser(ctx,
			&entity.ResolvedInvitation{ID: inv.ID, Roles: inv.Roles}, created.ID)
		if txErr != nil {
			return fmt.Errorf("claim invitation: %w", txErr)
		}

		return nil
	})
	if err != nil {
		reason := entity.AuditFailureUserProvisioning
		if errors.Is(err, apperr.ErrInvalidInvitation) {
			reason = entity.AuditFailureInvitationRefused
		}
		s.publishLoginFailure(ctx, invitee, meta, reason)

		return nil, err
	}

	pair, err := s.IssueTokenPair(ctx, user, cmd.ClientIP)
	if err != nil {
		s.publishLoginFailure(ctx, user, meta, issuanceFailureReason(err))

		return nil, fmt.Errorf("issue token pair: %w", err)
	}

	success := *meta
	success.SessionID = pair.SessionID.String()
	success.LoginMethod = entity.AuditLoginMethodPassword

	s.publishAudit(ctx, audit.LoginSuccess{User: user, Meta: &success})

	return pair, nil
}

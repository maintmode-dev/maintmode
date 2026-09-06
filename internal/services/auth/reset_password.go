package auth

import (
	"cmp"
	"context"
	"errors"
	"fmt"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xcripto"
)

// ResetPassword sets a password using a one-time code instead of the old one.
//
// The proof is possession of the mailbox rather than of the previous password,
// so EVERY session goes -- there is no session to spare, and someone resetting
// after losing control of an account wants exactly that.
//
// The policy check runs BEFORE the code is redeemed. Checking it after would
// make a policy failure prove the code was valid, and would consume an attempt
// on a request the user is about to retry with a longer password.
func (s *Service) ResetPassword(ctx context.Context, cmd *entity.ResetPasswordCmd) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.ResetPassword")
	defer span.End()

	if err := xcripto.ValidatePasswordPolicy(cmd.NewPassword); err != nil {
		// Indistinguishable from a bad code at the edge -- answering differently
		// would tell a caller holding a guessed code that the code was right.
		// The real reason goes to the audit trail, which the caller cannot read;
		// without this publish the distinction would exist nowhere at all.
		s.publishOTPLoginFailure(ctx, &entity.VerifyOTPCmd{
			Email: cmd.Email, ClientIP: cmd.ClientIP, UserAgent: cmd.UserAgent,
		}, nil, entity.AuditFailurePasswordPolicy)

		return apperr.ErrInvalidCredentials
	}

	user, reason, err := s.otpVerifier.Verify(ctx, &entity.VerifyOTPCmd{
		Email:        cmd.Email,
		Code:         cmd.Code,
		SessionNonce: cmd.SessionNonce,
		ClientIP:     cmd.ClientIP,
		UserAgent:    cmd.UserAgent,
	})
	if err != nil {
		// Audited unconditionally. Verify leaves the reason empty for an
		// infrastructural failure -- a database that did not answer while
		// resolving the user, reading the code, or consuming it -- and those
		// used to vanish from the trail entirely. A refused reset is worth
		// recording even when all that can be said is that something below
		// broke; the raw error stays out of the payload, which is a whitelist.
		s.publishOTPLoginFailure(ctx, &entity.VerifyOTPCmd{
			Email: cmd.Email, ClientIP: cmd.ClientIP, UserAgent: cmd.UserAgent,
		}, user, cmp.Or(reason, entity.AuditFailureUnknown))

		return err
	}

	// Verify does not refuse a blocked user: blocking is enforced inside
	// IssueAccessToken, which this path never calls because it issues no
	// tokens. Without this check a user blocked after their code was issued
	// could still install a password.
	if user.IsBlocked() {
		return apperr.ErrUserBlocked
	}

	// The write begins HERE, after Verify has returned -- installPassword opens
	// its transaction, and nothing wraps Verify. Wrapping it would change its
	// contract: it deliberately runs outside a transaction because issuance and
	// redemption would otherwise take the credential row and the new insert in
	// opposite lock orders.
	//
	// Consequence, accepted: if the write below fails the code is already
	// burnt and the user must request another. Burning a code on a failed write
	// is the safe direction -- the alternative is a code that survives its own
	// redemption.
	//
	// EVERY session goes: the proof was possession of the mailbox, not of a
	// session, so there is none to spare.
	err = s.installPassword(ctx, user.ID, cmd.NewPassword,
		func(txCtx context.Context) error {
			if txErr := s.tokenSrv.RevokeRefreshTokenByUserID(txCtx, user.ID); txErr != nil {
				return fmt.Errorf("revoke sessions: %w", txErr)
			}

			return nil
		})
	if err != nil {
		return err
	}

	s.publishAudit(ctx, audit.PasswordReset{
		User: user,
		Meta: &entity.AuditMetadata{IP: cmd.ClientIP, UserAgent: cmd.UserAgent},
	})
	return nil
}

// RequestPasswordReset issues a one-time code for a password reset, reusing the
// sign-in code mechanism unchanged.
//
// It answers the same way for every address, known or not: the nonce comes back
// regardless, so the response cannot be used to enumerate accounts.
func (s *Service) RequestPasswordReset(ctx context.Context, email string) (string, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.RequestPasswordReset")
	defer span.End()

	nonce, err := s.otpRequester.Request(ctx, email)
	if err != nil && !errors.Is(err, apperr.ErrUserNotFound) {
		return "", fmt.Errorf("request reset code: %w", err)
	}

	return nonce, nil
}

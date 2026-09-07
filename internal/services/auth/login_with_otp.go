package auth

import (
	"cmp"
	"context"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
)

// LoginWithOTP redeems an emailed one-time code and issues a token pair.
//
// It runs the same funnel as the password and OAuth methods — verify, then
// IssueTokenPair — and that is the reason it lives here rather than in the otp
// service. The blocked-user guard is inside IssueAccessToken, so a method
// reaching tokens this way inherits blocking, audit and IP binding without a new
// line. A second road to token issuance would lose all three at once.
//
// Every failure is audited, including the ones that happen before a user is
// known. That is the departure from the OAuth path, which lets pre-identification
// failures go unrecorded: there the equivalent failure is an unverifiable
// upstream token, which says nothing about a local account, while here it is
// someone trying codes against an address. A sweep across addresses that do not
// exist is exactly what this trail has to be able to show.
func (s *Service) LoginWithOTP(ctx context.Context, cmd *entity.VerifyOTPCmd) (*entity.TokenPair, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.LoginWithOTP")
	defer span.End()

	user, reason, err := s.otpVerifier.Verify(ctx, cmd)
	if err != nil {
		// Audited unconditionally. Verify leaves the reason empty for an
		// infrastructural failure -- a database error rather than a judged
		// credential -- and those used to be dropped on the grounds that they
		// are not really sign-in failures. But an attempt was made and refused,
		// and a trail that omits it reads as though nothing happened. The
		// distinction survives: it is recorded AS unknown rather than dressed up
		// as an invalid code.
		s.publishOTPLoginFailure(ctx, cmd, user, cmp.Or(reason, entity.AuditFailureUnknown))

		xlog.Warn(ctx, "otp login failed",
			xfield.String("client_ip", cmd.ClientIP),
			xfield.String("failure_reason", string(reason)),
			xfield.Error(err),
		)

		return nil, err
	}

	pair, err := s.IssueTokenPair(ctx, user, cmd.ClientIP)
	if err != nil {
		// A blocked user lands here: the guard inside IssueAccessToken refuses
		// the token even though the code was correct.
		s.publishLoginFailure(ctx, user,
			&entity.AuditMetadata{IP: cmd.ClientIP, UserAgent: cmd.UserAgent},
			entity.AuditFailureTokenIssuance)

		return nil, fmt.Errorf("issue token pair: %w", err)
	}

	s.publishAudit(ctx, audit.LoginSuccess{
		User: user,
		Meta: &entity.AuditMetadata{
			IP:        cmd.ClientIP,
			UserAgent: cmd.UserAgent,
			SessionID: pair.SessionID.String(),
		},
	})

	return pair, nil
}

// publishOTPLoginFailure records a rejected code.
//
// The user is nil when the address matched no account. The renderer dereferences
// the actor unconditionally, so the event carries a synthetic user holding the
// CLAIMED address — the only attribution an unidentified attempt has, and the
// one thing that makes a sweep findable afterwards.
func (s *Service) publishOTPLoginFailure(
	ctx context.Context,
	cmd *entity.VerifyOTPCmd,
	user *entity.User,
	reason entity.AuditFailureReason,
) {
	if user == nil {
		user = &entity.User{Email: cmd.Email}
	}

	s.publishLoginFailure(ctx, user,
		&entity.AuditMetadata{IP: cmd.ClientIP, UserAgent: cmd.UserAgent}, reason)
}

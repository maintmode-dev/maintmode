package auth

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
)

// LoginWithBreakGlass signs in to the break-glass account with the configured
// password, and nothing else: the account has no address anyone can use (see
// entity.BreakGlassEmail), so the password is the whole credential.
//
// Tokens come from IssueTokenPair like every other method's, so blocking, audit
// and IP binding apply without a line of their own. It reads no method flag:
// break-glass is the way in when every configured method is off or broken.
//
// The caller learns nothing about why a sign-in failed; the reason is in the
// audit record and the WARN log of the caller.
func (s *Service) LoginWithBreakGlass(
	ctx context.Context, cmd *entity.LoginWithBreakGlassCmd,
) (*entity.TokenPair, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.LoginWithBreakGlass")
	defer span.End()

	meta := &entity.AuditMetadata{
		IP:          cmd.ClientIP,
		UserAgent:   cmd.UserAgent,
		LoginMethod: entity.AuditLoginMethodBootstrap,
	}
	// The account by its reserved address: the actor of a failure before the
	// account is resolved, and the only one a failed attempt could be against.
	breakGlass := &entity.User{Email: entity.BreakGlassEmail, Name: entity.BreakGlassName}

	method, err := s.authMethods.Get(ctx, entity.AuthMethodBootstrap)
	if err != nil {
		return nil, fmt.Errorf("get auth method: %w", err)
	}

	claims, err := method.Authenticate(ctx, cmd.Password)
	if err != nil {
		s.publishLoginFailure(ctx, breakGlass, meta, entity.AuditFailureInvalidCredentials)

		return nil, apperr.ErrInvalidCredentials
	}

	// Possession of the break-glass secret is itself the authorization to
	// create the admin -- on a fresh instance it is the only way to get one.
	user, err := s.usersSrv.EnsureBreakGlassAccount(ctx, claims.Email, claims.Name)
	if err != nil {
		s.publishLoginFailure(ctx, breakGlass, meta, provisioningFailureReason(err))

		return nil, fmt.Errorf("ensure break-glass account: %w", err)
	}

	pair, err := s.IssueTokenPair(ctx, user, cmd.ClientIP)
	if err != nil {
		// A blocked break-glass account lands here: the guard inside
		// IssueAccessToken refuses the token, so blocking it switches
		// break-glass off.
		s.publishLoginFailure(ctx, user, meta, issuanceFailureReason(err))

		return nil, fmt.Errorf("issue token pair: %w", err)
	}

	success := *meta
	success.SessionID = pair.SessionID.String()
	s.publishAudit(ctx, audit.LoginSuccess{User: user, Meta: &success})

	return pair, nil
}

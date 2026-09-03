package auth

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
)

// LoginWithPassword signs a user in with a password and issues a token pair.
//
// Today the only method behind it is the break-glass bootstrap admin. It routes
// through the same funnel as the OAuth exchange — Authenticate →
// GetOrCreateByAuthInfo → IssueTokenPair — and that is the point: the
// blocked-user guard lives inside IssueAccessToken, so a method reaching tokens
// this way inherits blocking, audit and IP binding without a new line. A second
// road to token issuance would lose all three at once.
//
// The caller gets no detail about why a login failed: the endpoint collapses
// every failure into one identical response, so an attacker cannot learn
// whether the break-glass admin exists or what state it is in. The reason lives
// in the audit record and in the WARN log below — which is the only channel an
// operator has, since reading the audit log needs the admin session they are
// trying to recover.
func (s *Service) LoginWithPassword(ctx context.Context, cmd *entity.LoginWithPasswordCmd) (*entity.TokenPair, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.LoginWithPassword")
	defer span.End()

	pair, user, err := s.loginWithPassword(ctx, cmd)
	if err != nil {
		xlog.Warn(ctx, "password login failed",
			xfield.String("client_ip", cmd.ClientIP),
			xfield.Error(err),
		)
		return nil, err
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

func (s *Service) loginWithPassword(
	ctx context.Context,
	cmd *entity.LoginWithPasswordCmd,
) (*entity.TokenPair, *entity.User, error) {
	method, err := s.authMethods.Get(ctx, entity.AuthMethodBootstrap)
	if err != nil {
		return nil, nil, fmt.Errorf("get auth method: %w", err)
	}

	claims, err := method.Authenticate(ctx, cmd.Password)
	if err != nil {
		// Unlike the OAuth exchange, a credential mismatch IS audited here. There
		// the equivalent failure is an unverifiable upstream token, which says
		// nothing about a local account; a wrong break-glass password is an
		// attempt against a known admin credential and is the event this
		// permanently-live endpoint most needs on record.
		//
		// The email recorded is the one the request CLAIMED — Authenticate
		// returns no claims on failure, so it is the only attribution there is.
		s.publishPasswordLoginFailure(ctx, cmd, cmd.Email, entity.AuditFailureInvalidCredentials)
		return nil, nil, err
	}

	user, err := s.usersSrv.GetOrCreateByAuthInfo(ctx, entity.AuthMethodBootstrap, &entity.OAuthProviderUserInfo{
		ID:    claims.Subject,
		Email: claims.Email,
		Name:  claims.Name,
	}, entity.UserCreationPolicy{
		// Possession of the break-glass secret is itself the authorization to
		// create the admin. Without this the login would be refused with
		// ErrSignupDisabled on any instance that already has admins — which is
		// exactly the instance break-glass exists for, since it is needed when
		// those admins are unreachable.
		AllowCreate: true,
		GrantRoles:  []entity.Role{entity.RoleAdmin},
	})
	if err != nil {
		s.publishPasswordLoginFailure(ctx, cmd, claims.Email, provisioningFailureReason(err))
		return nil, nil, fmt.Errorf("get or create user: %w", err)
	}

	pair, err := s.IssueTokenPair(ctx, user, cmd.ClientIP)
	if err != nil {
		// A blocked bootstrap admin lands here: the guard inside IssueAccessToken
		// refuses the token, so blocking cuts off break-glass too.
		s.publishAudit(ctx, audit.LoginFailed{
			User: user,
			Meta: &entity.AuditMetadata{
				IP:            cmd.ClientIP,
				UserAgent:     cmd.UserAgent,
				FailureReason: entity.AuditFailureTokenIssuance,
			},
		})
		return nil, nil, fmt.Errorf("issue token pair: %w", err)
	}

	return pair, user, nil
}

// publishPasswordLoginFailure records a failure that happened before a user was
// resolved. The audit renderer dereferences the actor unconditionally, so the
// event carries a synthetic user rather than nil; its zero ID is the documented
// representation of "a login that failed before the user was known".
func (s *Service) publishPasswordLoginFailure(
	ctx context.Context,
	cmd *entity.LoginWithPasswordCmd,
	email string,
	reason entity.AuditFailureReason,
) {
	s.publishAudit(ctx, audit.LoginFailed{
		User: &entity.User{Email: email},
		Meta: &entity.AuditMetadata{
			IP:            cmd.ClientIP,
			UserAgent:     cmd.UserAgent,
			FailureReason: reason,
		},
	})
}

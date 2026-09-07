package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xcripto"
)

// decoyPasswordHash is verified against whenever no stored password is found,
// so that path costs the same as a real check. It is a fixed argon2id record of
// a value nothing can submit -- computed once at init rather than per request,
// which would double the cost of every miss.
var decoyPasswordHash = mustDecoyHash()

func mustDecoyHash() string {
	hash, err := xcripto.HashPassword("decoy-never-a-real-password")
	if err != nil {
		panic("build decoy password hash: " + err.Error())
	}

	return hash
}

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

// loginWithPassword resolves which credential answers for this address.
//
// The order is deliberate and is the whole behavior change of RUK-289:
//
//  1. A user with a stored password is verified against it. This is the
//     ordinary path and the ONLY path for anyone but the break-glass admin.
//  2. Otherwise, if the submitted address is the configured bootstrap one, the
//     break-glass password is tried -- but only while it is still live.
//  3. Otherwise the attempt fails like any other.
//
// Step 1 precedes step 2 so that once the admin has a password of their own, an
// unretired seed is not consulted for them: the personal credential wins as
// soon as it exists.
//
// Every path that does NOT verify a stored hash performs a decoy verification
// instead. argon2id costs tens of milliseconds and a lookup miss costs nothing,
// so without it the response time answers "does this account have a password?"
// -- and, for the bootstrap address, "is the seed still live?".
func (s *Service) loginWithPassword(
	ctx context.Context,
	cmd *entity.LoginWithPasswordCmd,
) (*entity.TokenPair, *entity.User, error) {
	user, err := s.usersSrv.GetByEmail(ctx, cmd.Email)
	if err != nil && !errors.Is(err, apperr.ErrUserNotFound) {
		return nil, nil, fmt.Errorf("look up user: %w", err)
	}

	if user != nil {
		pair, u, handled, credErr := s.loginWithStoredPassword(ctx, cmd, user)
		if handled {
			return pair, u, credErr
		}
	}

	return s.loginWithSeed(ctx, cmd)
}

// loginWithStoredPassword verifies against the user's own password. The handled
// flag says whether this path owns the outcome: a user without a password is
// not an answer here, they may still be the break-glass admin.
func (s *Service) loginWithStoredPassword(
	ctx context.Context,
	cmd *entity.LoginWithPasswordCmd,
	user *entity.User,
) (pair *entity.TokenPair, resolved *entity.User, handled bool, err error) {
	cred, err := s.passwords.GetPasswordByUserID(ctx, user.ID)
	if err != nil {
		if !errors.Is(err, apperr.ErrAuthCredentialNotFound) {
			return nil, nil, true, fmt.Errorf("read password credential: %w", err)
		}

		return nil, nil, false, nil
	}

	matches, err := xcripto.VerifyPassword(cred.SecretHash, cmd.Password)
	if err != nil {
		// A record this code cannot parse -- most likely a digest written by the
		// wrong hash function -- is a bug in whatever wrote it, not a bad
		// password. It fails the login and is audited as such rather than being
		// reported as a mismatch, which would bury it forever.
		xlog.Error(ctx, "stored password credential is unreadable", xfield.Error(err))
		s.publishPasswordLoginFailure(ctx, cmd, cmd.Email, entity.AuditFailureInvalidCredentials)

		return nil, nil, true, apperr.ErrInvalidCredentials
	}

	if !matches {
		s.publishPasswordLoginFailure(ctx, cmd, cmd.Email, entity.AuditFailureInvalidCredentials)
		return nil, nil, true, apperr.ErrInvalidCredentials
	}

	issued, err := s.IssueTokenPair(ctx, user, cmd.ClientIP)
	if err != nil {
		s.publishLoginFailure(ctx, user,
			&entity.AuditMetadata{IP: cmd.ClientIP, UserAgent: cmd.UserAgent},
			entity.AuditFailureTokenIssuance)

		return nil, nil, true, fmt.Errorf("issue token pair: %w", err)
	}

	return issued, user, true, nil
}

// loginWithSeed answers for the break-glass admin, and for every address that
// has no password at all.
func (s *Service) loginWithSeed(
	ctx context.Context,
	cmd *entity.LoginWithPasswordCmd,
) (*entity.TokenPair, *entity.User, error) {
	method, err := s.authMethods.Get(ctx, entity.AuthMethodBootstrap)
	if err != nil {
		return nil, nil, fmt.Errorf("get auth method: %w", err)
	}

	bootstrap, ok := method.(addressedMethod)
	if !ok {
		// A wiring bug, not a failed login: the registered break-glass provider
		// cannot report which address it serves. It answers like a wrong
		// address so the response stays uniform, but it must be visible.
		xlog.Error(ctx, "bootstrap auth method does not report its address")
	}

	if !ok || !strings.EqualFold(cmd.Email, bootstrap.Email()) {
		// Not the break-glass address. Pay the hashing cost anyway so this
		// answers as slowly as a real verification would.
		s.burnDecoyHash(ctx, cmd.Password)
		s.publishPasswordLoginFailure(ctx, cmd, cmd.Email, entity.AuditFailureInvalidCredentials)

		return nil, nil, apperr.ErrInvalidCredentials
	}

	claims, err := method.Authenticate(ctx, cmd.Password)
	if err != nil {
		// Pay the decoy cost here too. Authenticate is a constant-time compare
		// over a config string -- microseconds -- while every other failing
		// branch of this endpoint spends an argon2id verification. Without this
		// a wrong guess against the break-glass address answers measurably
		// faster than a wrong guess against any other address, which locates
		// the admin's address by timing alone. That is the one comparison the
		// decoy scheme exists to cover, and it was the branch it missed.
		s.burnDecoyHash(ctx, cmd.Password)

		// A wrong break-glass password is an attempt against a known admin
		// credential, and is the event this endpoint most needs on record.
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
		s.publishLoginFailure(ctx, user,
			&entity.AuditMetadata{IP: cmd.ClientIP, UserAgent: cmd.UserAgent},
			entity.AuditFailureTokenIssuance)

		return nil, nil, fmt.Errorf("issue token pair: %w", err)
	}

	return pair, user, nil
}

// addressedMethod is the part of the break-glass provider this path needs
// beyond the shared AuthMethod interface: which address it answers for. The
// submitted address selects the method, so a break-glass password sent against
// some other address must not sign anyone in.
type addressedMethod interface {
	Email() string
}

// burnDecoyHash spends the same work a real verification would, so a miss is
// not measurably faster than a match. The result is discarded by design.
func (s *Service) burnDecoyHash(ctx context.Context, password string) {
	if _, err := xcripto.VerifyPassword(decoyPasswordHash, password); err != nil {
		xlog.Error(ctx, "decoy password verification failed", xfield.Error(err))
	}
}

// publishPasswordLoginFailure records a failure that happened before a user was
// resolved. The claimed address is the only attribution such an attempt has.
func (s *Service) publishPasswordLoginFailure(
	ctx context.Context,
	cmd *entity.LoginWithPasswordCmd,
	email string,
	reason entity.AuditFailureReason,
) {
	s.publishLoginFailure(ctx, &entity.User{Email: email},
		&entity.AuditMetadata{IP: cmd.ClientIP, UserAgent: cmd.UserAgent}, reason)
}

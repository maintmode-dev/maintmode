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

	pair, user, method, err := s.loginWithPassword(ctx, cmd)
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
			IP:          cmd.ClientIP,
			UserAgent:   cmd.UserAgent,
			SessionID:   pair.SessionID.String(),
			LoginMethod: method,
		},
	})

	return pair, nil
}

// loginWithPassword resolves which credential answers for this address.
//
// The order is deliberate:
//
//  1. A user with a stored password is verified against it. This is the
//     ordinary path and the ONLY path for anyone but the break-glass admin.
//  2. Otherwise, if the submitted address is the configured bootstrap one, the
//     break-glass password is tried. Where one is configured it never stops
//     answering; where none is, every candidate is refused and the attempt is
//     indistinguishable from a wrong address.
//  3. Otherwise the attempt fails like any other.
//
// Step 1 precedes step 2 so that the personal credential wins as soon as it
// exists: setting a password is how an account stops going through break-glass,
// even though the break-glass credential itself is never retired.
//
// Every path that does NOT verify a stored hash performs a decoy verification
// instead. argon2id costs tens of milliseconds and a lookup miss costs nothing,
// so without it the response time answers "does this account have a password?"
// -- and, for the bootstrap address, whether the submitted address is the
// configured one.
func (s *Service) loginWithPassword(
	ctx context.Context,
	cmd *entity.LoginWithPasswordCmd,
) (*entity.TokenPair, *entity.User, entity.AuditLoginMethod, error) {
	user, err := s.usersSrv.GetByEmail(ctx, cmd.Email)
	if err != nil && !errors.Is(err, apperr.ErrUserNotFound) {
		return nil, nil, "", fmt.Errorf("look up user: %w", err)
	}

	if user != nil {
		pair, u, handled, credErr := s.loginWithStoredPassword(ctx, cmd, user)
		if handled {
			return pair, u, entity.AuditLoginMethodPassword, credErr
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
		s.publishPasswordLoginFailure(ctx, cmd, cmd.Email,
			entity.AuditFailureInvalidCredentials, entity.AuditLoginMethodPassword)

		return nil, nil, true, apperr.ErrInvalidCredentials
	}

	if !matches {
		s.publishPasswordLoginFailure(ctx, cmd, cmd.Email,
			entity.AuditFailureInvalidCredentials, entity.AuditLoginMethodPassword)
		return nil, nil, true, apperr.ErrInvalidCredentials
	}

	issued, err := s.IssueTokenPair(ctx, user, cmd.ClientIP)
	if err != nil {
		s.publishLoginFailure(ctx, user,
			&entity.AuditMetadata{
				IP:          cmd.ClientIP,
				UserAgent:   cmd.UserAgent,
				LoginMethod: entity.AuditLoginMethodPassword,
			},
			entity.AuditFailureTokenIssuance)

		return nil, nil, true, fmt.Errorf("issue token pair: %w", err)
	}

	return issued, user, true, nil
}

// loginWithSeed answers for the break-glass admin, and for every address that
// has no password at all.
//
// The name is a leftover from a one-time-seed design that was never merged:
// nothing here is seeded or spent, and the break-glass credential answers every
// time it is offered. Kept only because renaming an unexported function is
// churn against every open branch that touches this file.
func (s *Service) loginWithSeed(
	ctx context.Context,
	cmd *entity.LoginWithPasswordCmd,
) (*entity.TokenPair, *entity.User, entity.AuditLoginMethod, error) {
	method, err := s.authMethods.Get(ctx, entity.AuthMethodBootstrap)
	if err != nil {
		return nil, nil, "", fmt.Errorf("get auth method: %w", err)
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
		// Deliberately unlabeled: nothing verified here, and naming the method
		// would separate this from the branch below by record.
		s.publishPasswordLoginFailure(ctx, cmd, cmd.Email, entity.AuditFailureInvalidCredentials, "")

		return nil, nil, "", apperr.ErrInvalidCredentials
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
		//
		// Deliberately UNLABELLED, and this is the load-bearing one. The address
		// matched, but no credential verified. Labeling it would let anyone who
		// can read the audit log sort failed sign-ins by method and learn which
		// address the break-glass credential answers for -- on an instance where
		// it has never been used successfully, the trail is otherwise silent on
		// that. It would disclose by record exactly what the decoy verification
		// above spends an argon2id on hiding from timing.
		s.publishPasswordLoginFailure(ctx, cmd, cmd.Email, entity.AuditFailureInvalidCredentials, "")

		return nil, nil, "", err
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
		// Labeled, unlike the two branches above: the break-glass password
		// verified to reach this line. Nothing is disclosed either -- the actor
		// here is claims.Email, the CONFIGURED address, which this branch already
		// published before any of this.
		s.publishPasswordLoginFailure(ctx, cmd, claims.Email,
			provisioningFailureReason(err), entity.AuditLoginMethodBootstrap)

		return nil, nil, "", fmt.Errorf("get or create user: %w", err)
	}

	pair, err := s.IssueTokenPair(ctx, user, cmd.ClientIP)
	if err != nil {
		// A blocked bootstrap admin lands here: the guard inside IssueAccessToken
		// refuses the token, so blocking cuts off break-glass too. Labeled for
		// the same reason as the branch above: the credential verified.
		s.publishLoginFailure(ctx, user,
			&entity.AuditMetadata{
				IP:          cmd.ClientIP,
				UserAgent:   cmd.UserAgent,
				LoginMethod: entity.AuditLoginMethodBootstrap,
			},
			entity.AuditFailureTokenIssuance)

		return nil, nil, "", fmt.Errorf("issue token pair: %w", err)
	}

	return pair, user, entity.AuditLoginMethodBootstrap, nil
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
//
// method is empty for a failure where no credential verified. It is a parameter
// rather than something derived here because the distinction is per-branch and
// cannot be recovered from cmd: a wrong password against the break-glass address
// and a wrong password against any other address arrive identical.
func (s *Service) publishPasswordLoginFailure(
	ctx context.Context,
	cmd *entity.LoginWithPasswordCmd,
	email string,
	reason entity.AuditFailureReason,
	method entity.AuditLoginMethod,
) {
	s.publishLoginFailure(ctx, &entity.User{Email: email},
		&entity.AuditMetadata{
			IP:          cmd.ClientIP,
			UserAgent:   cmd.UserAgent,
			LoginMethod: method,
		}, reason)
}

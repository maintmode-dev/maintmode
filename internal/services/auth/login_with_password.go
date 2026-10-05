package auth

import (
	"context"
	"errors"
	"fmt"

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
// The caller gets no detail about why a login failed: the endpoint collapses
// every failure into one identical response, so an attacker cannot learn
// whether an account exists or what state it is in. The reason lives
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

// loginWithPassword verifies the address's stored password.
//
// Every path that does NOT verify a stored hash performs a decoy verification
// instead. argon2id costs tens of milliseconds and a lookup miss costs nothing,
// so without it the response time answers "does this account have a password?"
func (s *Service) loginWithPassword(
	ctx context.Context,
	cmd *entity.LoginWithPasswordCmd,
) (*entity.TokenPair, *entity.User, entity.AuditLoginMethod, error) {
	user, err := s.usersSrv.GetByEmail(ctx, cmd.Email)
	if err != nil && !errors.Is(err, apperr.ErrUserNotFound) {
		return nil, nil, "", fmt.Errorf("look up user: %w", err)
	}

	// The gate guards ENTRY to the stored-password step rather than living
	// inside it: no stored hash is ever compared while the method is off, so
	// the outcome is identical for a right and a wrong password in response, in
	// timing, and in the audit record.
	if user != nil && s.methodOffered(ctx, entity.AuthMethodNameEmailPassword, cmd.ClientIP) {
		pair, u, handled, credErr := s.loginWithStoredPassword(ctx, cmd, user)
		if handled {
			return pair, u, entity.AuditLoginMethodPassword, credErr
		}
	}

	s.burnDecoyHash(ctx, cmd.Password)
	// Unlabeled: no credential verified.
	s.publishPasswordLoginFailure(ctx, cmd, cmd.Email, entity.AuditFailureInvalidCredentials, "")

	return nil, nil, "", apperr.ErrInvalidCredentials
}

// loginWithStoredPassword verifies against the user's own password. The handled
// flag says whether this path owns the outcome: a user without a password is
// not an answer here.
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
			issuanceFailureReason(err))

		return nil, nil, true, fmt.Errorf("issue token pair: %w", err)
	}

	return issued, user, true, nil
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
// cannot be recovered from cmd.
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

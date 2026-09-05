package otp

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/metrics"
	"github.com/ruko1202/maintmode/internal/utils/xhash"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

// Verify redeems a one-time code and reports the user it belongs to.
//
// It returns the resolved user rather than a token pair: issuing tokens is the
// auth service's job, and routing this through the same IssueTokenPair funnel as
// every other method is what makes blocking, audit and IP binding apply here
// without restating them.
//
// Every failure except one answers the same way to the caller. The exception is
// ErrOTPSessionMismatch, argued at its definition. The reason for each failure
// reaches the audit trail through the returned FailureReason, which is where the
// distinctions live once the response has flattened them.
//
// This method takes NO transaction, and that is load-bearing rather than an
// omission. Its statements each take one row lock and never hold one while
// taking another, while issuance holds its lock across a KMS call and a queue
// insert. Wrapping these steps in a transaction -- the natural reading, and the
// house style for multi-statement work -- would have the two paths acquiring the
// credential row and the new insert in opposite orders.
func (s *Service) Verify(ctx context.Context, cmd *entity.VerifyOTPCmd) (*entity.User, entity.AuditFailureReason, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.OTP.Verify")
	defer span.End()

	user, err := s.userSrv.GetByEmail(ctx, cmd.Email)
	switch {
	case errors.Is(err, apperr.ErrUserNotFound):
		// Audited with the claimed address and no user id. This is the case the
		// ticket cares most about: a sweep across addresses that do not exist is
		// invisible unless the attempt is recorded here.
		return nil, entity.AuditFailureInvalidCode, apperr.ErrInvalidCredentials
	case err != nil:
		return nil, "", fmt.Errorf("resolve user by email: %w", err)
	}

	cred, err := s.store.GetUnconsumedOTPByUserID(ctx, user.ID)
	switch {
	case errors.Is(err, apperr.ErrAuthCredentialNotFound):
		// No live code. Deliberately not its own reason: to the caller it is
		// indistinguishable from a wrong code, and a separate audit value would
		// record a distinction the attacker cannot make either.
		return user, entity.AuditFailureInvalidCode, apperr.ErrInvalidCredentials
	case err != nil:
		return nil, "", fmt.Errorf("look up live otp: %w", err)
	}

	if expired(cred) {
		// Consumed so the dead row stops occupying the single active-OTP slot,
		// which is what keeps the reissue barrier bounded by the code's own
		// lifetime. The result is ignored on purpose: a false means a concurrent
		// caller retired the same row first, and either way the slot is free.
		// Failing the request on that bookkeeping would turn a lost race into a
		// user-visible error, and nothing about correctness rests on this caller
		// being the one that won.
		consumed, err := s.store.ConsumeOTP(ctx, cred.ID)
		if err != nil {
			// Counted, not just logged. A persistent failure here is worse than
			// a lost race: claimSlot retires expired codes through this same
			// statement, so a code that is both expired and burnt would keep the
			// slot forever, barring the user from ever receiving a new one. The
			// "bounded by the code's own lifetime" guarantee rests on this write
			// succeeding, so a failure has to be visible rather than inferred.
			metrics.OTPRetireExpiredError(ctx)
			xlog.Error(ctx, "failed to retire an expired otp",
				xfield.String("credential_id", cred.ID.String()),
				xfield.Error(err),
			)
		}

		if !consumed {
			// The row was already retired by a concurrent caller, so this
			// submission found nothing to expire. Reported as an invalid code
			// rather than as an expiry: it is the same thing the NEXT submission
			// would see, and reporting it as an expiry would let a caller keep
			// writing `code expired` audit rows against a known address by
			// replaying past the retirement. Every other reason is bounded by
			// the attempt ceiling; this branch has no such bound, so it must not
			// be reachable more than once per code.
			return user, entity.AuditFailureInvalidCode, apperr.ErrInvalidCredentials
		}

		return user, entity.AuditFailureCodeExpired, apperr.ErrInvalidCredentials
	}

	// The claim comes BEFORE either comparison, and is the ceiling check as well
	// as the increment. Splitting them would cap recorded attempts rather than
	// performed ones -- see ClaimOTPAttempt.
	claimed, err := s.store.ClaimOTPAttempt(ctx, cred.ID, s.maxAttempts)
	if err != nil {
		// Fails the request rather than proceeding. Comparing a guess that was
		// never counted is exactly the bypass the claim exists to close: an
		// attacker able to induce claim failures would otherwise get unlimited
		// free guesses. The counter is what makes a persistent failure visible,
		// since nothing in this stack alerts on logs.
		metrics.OTPAttemptClaimError(ctx)
		xlog.Error(ctx, "failed to claim an otp attempt",
			xfield.String("credential_id", cred.ID.String()),
			xfield.Error(err),
		)

		return nil, "", fmt.Errorf("claim otp attempt: %w", err)
	}

	if !claimed {
		// The ceiling is spent. The code is NOT consumed here: leaving it in
		// place is what bars a fresh one until it expires on its own.
		return user, entity.AuditFailureAttemptsExhausted, apperr.ErrInvalidCredentials
	}

	// Nonce before code. Both are attacker-supplied and both are compared in
	// constant time, so neither order leaks by timing; checking the nonce first
	// means a user who lost their tab gets the actionable message even when they
	// also fat-fingered the code.
	if !matches(cred.SessionNonce, cmd.SessionNonce) {
		return user, entity.AuditFailureSessionMismatch, apperr.ErrOTPSessionMismatch
	}

	if !matches(&cred.SecretHash, xhash.HashSha256([]byte(cmd.Code))) {
		return user, entity.AuditFailureInvalidCode, apperr.ErrInvalidCredentials
	}

	consumed, err := s.store.ConsumeOTP(ctx, cred.ID)
	if err != nil {
		return nil, "", fmt.Errorf("consume otp: %w", err)
	}

	if !consumed {
		// Someone else claimed this code between the comparison and here. This
		// caller had the right code and lost a race, so the reason is imprecise
		// -- their sibling holds the token pair. Rare, harmless, and not worth a
		// fifth reason whose only reader is someone already looking at a matched
		// pair of rows one millisecond apart.
		return user, entity.AuditFailureInvalidCode, apperr.ErrInvalidCredentials
	}

	return user, "", nil
}

// expired reports whether a code is past its life. A nil ExpiresAt is treated as
// expired rather than eternal: the column is nullable because a password row has
// no expiry, and a code that reached this point without one is a bug whose safe
// reading is "dead", not "valid forever".
func expired(cred *entity.AuthCredential) bool {
	return cred.ExpiresAt == nil || !cred.ExpiresAt.After(xtime.UTCNow())
}

// matches compares a stored secret against what the caller sent, in constant
// time. ConstantTimeCompare returns 0 for unequal lengths, so no length check is
// needed -- and adding one would reintroduce the branch this avoids.
//
// A nil stored value never matches: it means the row carries no nonce at all,
// which for a one-time code is a row that should not exist.
func matches(stored *string, given string) bool {
	if stored == nil {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(*stored), []byte(given)) == 1
}

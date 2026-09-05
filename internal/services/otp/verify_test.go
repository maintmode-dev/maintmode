package otp_test

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// issueCode drives a real request and digs the code out of the queued task, so
// the verify tests redeem exactly what a user would have been emailed.
func issueCode(ctx context.Context, t *testing.T, svc *otpService) (user *entity.User, code, nonce string) {
	t.Helper()

	user = makeUser(ctx, t)

	nonce, err := svc.svc.Request(ctx, user.Email)
	require.NoError(t, err)

	return user, svc.decodeCode(t), nonce
}

func TestVerify_RedeemsACorrectCode(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc := newVerifyService(t)
	user, code, nonce := issueCode(ctx, t, svc)

	got, reason, err := svc.svc.Verify(ctx, &entity.VerifyOTPCmd{
		Email: user.Email, Code: code, SessionNonce: nonce,
	})
	require.NoError(t, err)
	require.Empty(t, reason)
	require.Equal(t, user.ID, got.ID)

	// Single use: the row is consumed, so the same code cannot be redeemed twice.
	_, err = credStore.GetUnconsumedOTPByUserID(ctx, user.ID)
	require.ErrorIs(t, err, apperr.ErrAuthCredentialNotFound)

	_, reason, err = svc.svc.Verify(ctx, &entity.VerifyOTPCmd{
		Email: user.Email, Code: code, SessionNonce: nonce,
	})
	require.ErrorIs(t, err, apperr.ErrInvalidCredentials)
	require.Equal(t, entity.AuditFailureInvalidCode, reason)
}

func TestVerify_RejectsAWrongCodeAndCountsTheAttempt(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc := newVerifyService(t)
	user, code, nonce := issueCode(ctx, t, svc)

	wrong := "000000"
	if code == wrong {
		wrong = "111111"
	}

	_, reason, err := svc.svc.Verify(ctx, &entity.VerifyOTPCmd{
		Email: user.Email, Code: wrong, SessionNonce: nonce,
	})
	require.ErrorIs(t, err, apperr.ErrInvalidCredentials)
	require.Equal(t, entity.AuditFailureInvalidCode, reason)

	cred, err := credStore.GetUnconsumedOTPByUserID(ctx, user.ID)
	require.NoError(t, err)
	require.EqualValues(t, 1, cred.Attempts, "a submitted guess must be counted")
	require.Nil(t, cred.ConsumedAt, "a wrong guess must not retire the code")
}

// TestVerify_StopsAtTheCeiling walks the ceiling down and checks the code is
// still NOT consumed afterwards. That second assertion is the one that matters:
// leaving the burnt row in place is what bars a fresh code until it expires.
func TestVerify_StopsAtTheCeiling(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc := newVerifyService(t)
	user, code, nonce := issueCode(ctx, t, svc)

	for range svc.svc.MaxAttempts() {
		_, reason, err := svc.svc.Verify(ctx, &entity.VerifyOTPCmd{
			Email: user.Email, Code: "000000", SessionNonce: nonce,
		})
		require.ErrorIs(t, err, apperr.ErrInvalidCredentials)
		require.Equal(t, entity.AuditFailureInvalidCode, reason)
	}

	// Past the ceiling the reason changes, and the CORRECT code no longer works:
	// the ceiling is checked before any comparison.
	_, reason, err := svc.svc.Verify(ctx, &entity.VerifyOTPCmd{
		Email: user.Email, Code: code, SessionNonce: nonce,
	})
	require.ErrorIs(t, err, apperr.ErrInvalidCredentials)
	require.Equal(t, entity.AuditFailureAttemptsExhausted, reason)

	cred, err := credStore.GetUnconsumedOTPByUserID(ctx, user.ID)
	require.NoError(t, err)
	require.Nil(t, cred.ConsumedAt, "a burnt code must keep the slot")
	require.Equal(t, svc.svc.MaxAttempts(), cred.Attempts)
}

// TestVerify_WrongNonceIsItsOwnError pins the one failure that does not collapse
// into the generic answer, and TestVerify_WrongNonceAndWrongCode pins the order:
// nonce is checked first, so a user who lost their tab is told to ask for a new
// code even when they also mistyped.
func TestVerify_WrongNonceIsItsOwnError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc := newVerifyService(t)
	user, code, _ := issueCode(ctx, t, svc)

	_, reason, err := svc.svc.Verify(ctx, &entity.VerifyOTPCmd{
		Email: user.Email, Code: code, SessionNonce: uuid.NewString(),
	})
	require.ErrorIs(t, err, apperr.ErrOTPSessionMismatch)
	require.Equal(t, entity.AuditFailureSessionMismatch, reason)
}

func TestVerify_WrongNonceAndWrongCode(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc := newVerifyService(t)
	user, _, _ := issueCode(ctx, t, svc)

	_, reason, err := svc.svc.Verify(ctx, &entity.VerifyOTPCmd{
		Email: user.Email, Code: "000000", SessionNonce: uuid.NewString(),
	})
	require.ErrorIs(t, err, apperr.ErrOTPSessionMismatch,
		"the nonce is checked before the code, so this is the actionable failure")
	require.Equal(t, entity.AuditFailureSessionMismatch, reason)
}

func TestVerify_EmptyNonceIsAMismatch(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc := newVerifyService(t)
	user, code, _ := issueCode(ctx, t, svc)

	_, reason, err := svc.svc.Verify(ctx, &entity.VerifyOTPCmd{
		Email: user.Email, Code: code, SessionNonce: "",
	})
	require.ErrorIs(t, err, apperr.ErrOTPSessionMismatch)
	require.Equal(t, entity.AuditFailureSessionMismatch, reason)
}

// TestVerify_ExpiredCodeIsRetired checks both halves: the caller is refused with
// the expiry reason, and the dead row is consumed so it stops occupying the
// single active-OTP slot.
func TestVerify_ExpiredCodeIsRetired(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc := newVerifyService(t)
	user := makeUser(ctx, t)
	expired := makeExpiredOTP(ctx, t, user.ID)

	_, reason, err := svc.svc.Verify(ctx, &entity.VerifyOTPCmd{
		Email: user.Email, Code: "000000", SessionNonce: *expired.SessionNonce,
	})
	require.ErrorIs(t, err, apperr.ErrInvalidCredentials)
	require.Equal(t, entity.AuditFailureCodeExpired, reason)

	_, err = credStore.GetUnconsumedOTPByUserID(ctx, user.ID)
	require.ErrorIs(t, err, apperr.ErrAuthCredentialNotFound,
		"an expired code must not keep the slot")
}

// TestVerify_UnknownAddressIsAudited is the case the ticket most wants visible:
// a sweep across addresses that do not exist. It must be refused like everything
// else AND carry a reason, since there is no user row to hang it on.
func TestVerify_UnknownAddressIsAudited(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc := newVerifyService(t)

	user, reason, err := svc.svc.Verify(ctx, &entity.VerifyOTPCmd{
		Email: uuid.NewString() + "@example.com", Code: "000000", SessionNonce: uuid.NewString(),
	})
	require.ErrorIs(t, err, apperr.ErrInvalidCredentials)
	require.Equal(t, entity.AuditFailureInvalidCode, reason)
	require.Nil(t, user, "there is no user to attribute an unknown address to")
}

func TestVerify_NoLiveCode(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc := newVerifyService(t)
	user := makeUser(ctx, t)

	_, reason, err := svc.svc.Verify(ctx, &entity.VerifyOTPCmd{
		Email: user.Email, Code: "000000", SessionNonce: uuid.NewString(),
	})
	require.ErrorIs(t, err, apperr.ErrInvalidCredentials)
	require.Equal(t, entity.AuditFailureInvalidCode, reason,
		"indistinguishable from a wrong code, by design")
}

// TestVerify_ConcurrentRedemptionYieldsOneWinner is the single-use guarantee
// under contention. Both goroutines hold the same correct code and are released
// from one barrier; exactly one may come away with the user.
func TestVerify_ConcurrentRedemptionYieldsOneWinner(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc := newVerifyService(t)
	user, code, nonce := issueCode(ctx, t, svc)

	const racers = 2

	start := make(chan struct{})
	users := make([]*entity.User, racers)
	errs := make([]error, racers)

	var wg sync.WaitGroup
	for i := range users {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			users[i], _, errs[i] = svc.svc.Verify(ctx, &entity.VerifyOTPCmd{
				Email: user.Email, Code: code, SessionNonce: nonce,
			})
		}()
	}

	close(start)
	wg.Wait()

	winners := 0
	for i := range users {
		if errs[i] == nil {
			winners++
			require.Equal(t, user.ID, users[i].ID)
		} else {
			require.ErrorIs(t, errs[i], apperr.ErrInvalidCredentials)
		}
	}

	require.Equal(t, 1, winners, "a code is redeemable exactly once")
}

package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap/zaptest"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

// stubMethodFlags answers the built-in flags from a fixed map, or fails, to
// drive the unreadable path.
type stubMethodFlags struct {
	enabled map[entity.AuthMethodName]bool
	fail    bool
}

func (s stubMethodFlags) Enabled(_ context.Context, method entity.AuthMethodName) (bool, error) {
	if s.fail {
		return false, errors.New("auth_settings unreadable")
	}

	return s.enabled[method], nil
}

func flagsWith(enabled map[entity.AuthMethodName]bool) stubMethodFlags {
	return stubMethodFlags{enabled: enabled}
}

// Criterion 4: a user with a CORRECT stored password is refused when
// email_password is off.
//
// The correct password is the whole point of the assertion. Against a wrong one
// the test would pass with no gate at all, since the password would be refused
// anyway -- so it would prove nothing about this feature.
func TestLoginWithPassword_RefusedWhenMethodDisabled(t *testing.T) {
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	srv, _ := initServiceWithUnrelatedBootstrapFlags(t, flagsWith(map[entity.AuthMethodName]bool{
		entity.AuthMethodNameEmailPassword: false,
	}))
	password := "correct-horse-" + xuuid.NewString()
	user := makePasswordUser(ctx, t, srv, password)

	_, err := srv.LoginWithPassword(ctx, &entity.LoginWithPasswordCmd{
		Email:    user.Email,
		Password: password,
	})
	require.ErrorIs(t, err, apperr.ErrInvalidCredentials,
		"a correct password must not sign anyone in while the method is off")
}

// Criterion 15: an unreadable flag refuses ordinary sign-in and leaves
// break-glass working.
//
// The ordinary user has a CORRECT password: against a wrong one the refusal
// proves nothing about the gate.
func TestLoginWithPassword_UnreadableFlagFailsClosedButKeepsBreakGlass(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	breakGlass := "the-break-glass-" + xuuid.NewString()
	srv, _ := initServiceWithBreakGlass(t, newTestBreakGlass(breakGlass), stubMethodFlags{fail: true})
	personal := "personal-" + xuuid.NewString()
	user := makePasswordUser(ctx, t, srv, personal)

	_, err := srv.LoginWithPassword(ctx, &entity.LoginWithPasswordCmd{
		Email:    user.Email,
		Password: personal,
	})
	require.ErrorIs(t, err, apperr.ErrInvalidCredentials,
		"an unreadable flag must fail closed, not open")

	pair, err := srv.LoginWithBreakGlass(ctx, &entity.LoginWithBreakGlassCmd{Password: breakGlass})
	require.NoError(t, err, "break-glass does not read the table, so it must survive its outage")
	require.NotEmpty(t, pair.AccessToken)
}

// Criterion 6: with the method off, a right and a wrong password are
// indistinguishable -- same error, and the same audit reason.
//
// The audit half is the one that needs asserting. The gate skips the stored
// password entirely, so nothing compares it; had it refused after comparing,
// the reason would have partitioned addresses into "has a valid password" and
// "does not" for anyone who can read the audit log.
func TestLoginWithPassword_DisabledRefusalDoesNotRevealThePassword(t *testing.T) {
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	srv, _ := initServiceWithUnrelatedBootstrapFlags(t, flagsWith(map[entity.AuthMethodName]bool{
		entity.AuthMethodNameEmailPassword: false,
	}))
	password := "correct-horse-" + xuuid.NewString()
	user := makePasswordUser(ctx, t, srv, password)

	spy := spyOnFailures(srv)
	counter := countCredentialReads(srv)

	_, rightErr := srv.LoginWithPassword(ctx, &entity.LoginWithPasswordCmd{
		Email:    user.Email,
		Password: password,
	})

	_, wrongErr := srv.LoginWithPassword(ctx, &entity.LoginWithPasswordCmd{
		Email:    user.Email,
		Password: "definitely-not-" + xuuid.NewString(),
	})

	require.Equal(t, rightErr, wrongErr)
	require.Len(t, spy.reasons, 2)
	require.Equal(t, spy.reasons[0], spy.reasons[1],
		"the audit reason must not depend on whether the password was right")

	// The ORDINARY reason, not a disabled-method one:
	// a disabled method does not get to vary it. The
	// refusal is reported on the caller's log line instead.
	require.Equal(t, entity.AuditFailureInvalidCredentials, spy.reasons[0])

	// The clause that makes the rest of this test mean something: the stored
	// hash was never READ, let alone compared.
	//
	// Without it the test is green against a gate placed INSIDE the
	// stored-password step that refuses after comparing -- both refusals would
	// still answer with the same reason, so the
	// response and the audit record would look identical while the credential
	// was being read on every attempt. That is the misplacement the gate's
	// placement exists to avoid, and this is the only assertion that sees it.
	require.Zero(t, counter.reads, "a disabled method must not read the stored credential at all")
}

// reasonSpy records the failure reason of every audited login failure.
//
// The package's usual harness publishes into the real goque queue, which is
// right for tests about auth flows and useless here: this one is about WHICH
// reason was recorded, so it needs to read them back.
type reasonSpy struct {
	reasons []entity.AuditFailureReason
}

func (s *reasonSpy) Publish(_ context.Context, action audit.Action) error {
	if failed, ok := action.(audit.LoginFailed); ok && failed.Meta != nil {
		s.reasons = append(s.reasons, failed.Meta.FailureReason)
	}

	return nil
}

// spyOnFailures swaps the service's publisher for one that records reasons.
// Same package, unexported field -- no seam needed.
func spyOnFailures(srv *Service) *reasonSpy {
	spy := &reasonSpy{}
	srv.auditPublisher = spy

	return spy
}

// Criterion 8: changing your own password still works while password SIGN-IN is
// disabled.
//
// The two are different operations and only one of them is a way in. An admin
// who turns off password sign-in has not asked to freeze every existing
// credential in place -- that would strip the ability to retire a password
// rather than to use one, which is the opposite of the intent.
func TestChangePassword_SurvivesMethodDisabled(t *testing.T) {
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	srv, _ := initServiceWithUnrelatedBootstrapFlags(t, flagsWith(map[entity.AuthMethodName]bool{
		entity.AuthMethodNameEmailPassword: false,
	}))
	current := "current-" + xuuid.NewString()
	user := makePasswordUser(ctx, t, srv, current)

	pair, err := srv.IssueTokenPair(ctx, user, "10.0.0.1")
	require.NoError(t, err)

	require.NoError(t, srv.ChangePassword(ctx, &entity.ChangePasswordCmd{
		UserID:          user.ID,
		CurrentPassword: current,
		NewPassword:     "replacement-" + xuuid.NewString(),
		RefreshToken:    pair.RefreshToken,
	}), "disabling password sign-in must not freeze existing passwords against change")
}

// Criterion 9: disabling a method does not end sessions already open.
//
// Decided by the design: only NEW sign-ins close. This is a regression guard
// rather than a feature test -- it is green against an implementation that does
// nothing, which is correct, because doing nothing is the specified behavior.
// It exists so a later change cannot quietly add revocation.
func TestDisablingAMethod_DoesNotEndExistingSessions(t *testing.T) {
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	// The flag source is the admin's switch: both methods on while the session
	// opens, both off afterwards -- the same table flip the settings API makes.
	switched := map[entity.AuthMethodName]bool{
		entity.AuthMethodNameEmailPassword: true,
		entity.AuthMethodNameEmailOTP:      true,
	}
	srv, _ := initServiceWithUnrelatedBootstrapFlags(t, flagsWith(switched))
	user := makePasswordUser(ctx, t, srv, "pw-"+xuuid.NewString())

	pair, err := srv.IssueTokenPair(ctx, user, "10.0.0.1")
	require.NoError(t, err)

	switched[entity.AuthMethodNameEmailPassword] = false
	switched[entity.AuthMethodNameEmailOTP] = false

	// The token issued before the change still authenticates afterwards.
	require.NoError(t, srv.EnsureActiveToken(ctx, pair.AccessToken),
		"a session open before the change must survive it")
}

// Criterion 18: with every built-in off and no working provider, break-glass
// still gets in.
//
// Nothing stops an admin reaching that state -- there is no last-method guard,
// deliberately -- so this is not an edge case to be argued about but the
// configuration the removal of that guard relies on being survivable. Hence a
// test rather than a comment.
func TestBreakGlass_SurvivesEverythingDisabled(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	breakGlass := "the-break-glass-" + xuuid.NewString()
	srv, _ := initServiceWithBreakGlass(t, newTestBreakGlass(breakGlass), flagsWith(map[entity.AuthMethodName]bool{
		entity.AuthMethodNameEmailPassword: false,
		entity.AuthMethodNameEmailOTP:      false,
	}))

	pair, err := srv.LoginWithBreakGlass(ctx, &entity.LoginWithBreakGlassCmd{Password: breakGlass})
	require.NoError(t, err, "break-glass is the documented recovery from a total lockout")
	require.NotEmpty(t, pair.AccessToken)
}

// credentialReadCounter wraps the password store and counts lookups.
type credentialReadCounter struct {
	PasswordCredentials

	reads int
}

func (c *credentialReadCounter) GetPasswordByUserID(
	ctx context.Context,
	userID uuid.UUID,
) (*entity.AuthCredential, error) {
	c.reads++

	return c.PasswordCredentials.GetPasswordByUserID(ctx, userID)
}

// countCredentialReads makes the service's credential lookups observable.
//
// Same-package, unexported field: no production seam is added for a test.
func countCredentialReads(srv *Service) *credentialReadCounter {
	counter := &credentialReadCounter{PasswordCredentials: srv.passwords}
	srv.passwords = counter

	return counter
}

// A password installed through RESET is still refused when email_password is
// off.
//
// This is the branch that decides how bad the ungated reset path is. Reset
// keeps issuing codes while email_otp is disabled (a documented consequence --
// see SPEC 4.4.1), so someone with mailbox access can set a password. What
// stops that becoming a way in is this: the gate skips the stored-credential
// step entirely rather than judging the credential, so a freshly installed
// password is refused exactly like any other.
//
// Were it otherwise -- were the gate to judge the credential rather than skip
// it -- the ungated reset path would be an escalation rather than a mental-model
// gap.
func TestLoginWithPassword_ResetInstalledPasswordIsStillRefused(t *testing.T) {
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	srv, mocks := initServiceWithUnrelatedBootstrapFlags(t, flagsWith(map[entity.AuthMethodName]bool{
		entity.AuthMethodNameEmailOTP:      false,
		entity.AuthMethodNameEmailPassword: false,
	}))
	user := makePasswordUser(ctx, t, srv, "old-"+xuuid.NewString())

	// Recovery still works with email codes off, which is the point of 4.4.
	mocks.otpVerifier.EXPECT().
		Verify(gomock.Any(), gomock.Any()).
		Return(user, entity.AuditFailureReason(""), nil).
		Times(1)

	installed := "attacker-chosen-" + xuuid.NewString()
	require.NoError(t, srv.ResetPassword(ctx, &entity.ResetPasswordCmd{
		Email:        user.Email,
		Code:         "123456",
		SessionNonce: xuuid.NewString(),
		NewPassword:  installed,
	}))

	// ...and the password it installed buys nothing while the method is off.
	_, err := srv.LoginWithPassword(ctx, &entity.LoginWithPasswordCmd{
		Email:    user.Email,
		Password: installed,
	})
	require.ErrorIs(t, err, apperr.ErrInvalidCredentials,
		"a password set through recovery must not bypass a disabled method")
}

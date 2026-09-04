package bootstrapauth_test

import (
	"context"
	"strings"
	"testing"

	"github.com/ruko1202/xlog"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/services/authmethod/bootstrapauth"
)

const testEmail = "admin@example.com"

// observedCtx returns a context whose logger writes into the returned sink, so
// a test can assert on what did — and did not — reach the log.
func observedCtx(t *testing.T) (context.Context, *observer.ObservedLogs) {
	t.Helper()

	core, logs := observer.New(zapcore.DebugLevel)
	return xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zap.New(core))), logs
}

// newService builds the break-glass method with the shared test email and the
// given resolved password.
func newService(password string) *bootstrapauth.Service {
	return bootstrapauth.NewService(config.BootstrapConfig{Email: testEmail}, password)
}

func loggedText(logs *observer.ObservedLogs) string {
	var sb strings.Builder
	for _, entry := range logs.All() {
		sb.WriteString(entry.Message)
		for _, f := range entry.Context {
			sb.WriteString(" ")
			sb.WriteString(f.String)
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func TestResolvePassword(t *testing.T) {
	t.Parallel()

	t.Run("a configured password is returned and never logged", func(t *testing.T) {
		t.Parallel()

		const configured = "a-configured-break-glass-password"
		ctx, logs := observedCtx(t)

		got, err := bootstrapauth.ResolvePassword(ctx, config.BootstrapConfig{Password: configured})
		require.NoError(t, err)
		require.Equal(t, configured, got)
		require.NotContains(t, loggedText(logs), configured,
			"a configured password must never reach the log")
	})

	t.Run("an empty password is generated and logged exactly once", func(t *testing.T) {
		t.Parallel()

		ctx, logs := observedCtx(t)

		got, err := bootstrapauth.ResolvePassword(ctx, config.BootstrapConfig{})
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(got), 22,
			"a generated password must carry at least 128 bits of entropy")

		carrying := 0
		for _, entry := range logs.All() {
			if strings.Contains(entry.Message, got) {
				carrying++
				continue
			}
			for _, f := range entry.Context {
				if strings.Contains(f.String, got) {
					carrying++
					break
				}
			}
		}
		require.Equal(t, 1, carrying, "the generated password must be logged exactly once")
	})

	t.Run("two generated passwords differ", func(t *testing.T) {
		t.Parallel()

		ctx, _ := observedCtx(t)

		first, err := bootstrapauth.ResolvePassword(ctx, config.BootstrapConfig{})
		require.NoError(t, err)
		second, err := bootstrapauth.ResolvePassword(ctx, config.BootstrapConfig{})
		require.NoError(t, err)
		require.NotEqual(t, first, second)
	})
}

func TestServiceMethodID(t *testing.T) {
	t.Parallel()

	svc := newService("pw")
	require.Equal(t, entity.AuthMethodBootstrap, svc.MethodID())
}

func TestServiceAuthenticate(t *testing.T) {
	t.Parallel()

	const password = "the-break-glass-password"

	t.Run("the right password yields the configured identity", func(t *testing.T) {
		t.Parallel()

		ctx, _ := observedCtx(t)
		svc := newService(password)

		claims, err := svc.Authenticate(ctx, password)
		require.NoError(t, err)
		require.Equal(t, entity.BootstrapSubject, claims.Subject)
		require.Equal(t, testEmail, claims.Email)
		require.NotEmpty(t, claims.Name)
	})

	t.Run("the subject is constant so a repeat login resolves the same user", func(t *testing.T) {
		t.Parallel()

		ctx, _ := observedCtx(t)
		svc := newService(password)

		first, err := svc.Authenticate(ctx, password)
		require.NoError(t, err)
		second, err := svc.Authenticate(ctx, password)
		require.NoError(t, err)
		require.Equal(t, first.Subject, second.Subject)
	})

	// Every wrong password yields the same error, whatever its shape: same
	// length as the real one, shorter, a prefix-plus-suffix, empty.
	//
	// This says nothing about TIMING. A plain `!=` passes every assertion here,
	// so the constant-time property is not covered by this test and cannot
	// honestly be — a wall-clock assertion on a single comparison is noise on
	// any real machine. That property is a review-checklist item instead:
	// subtle.ConstantTimeCompare must be the only comparison of the credential
	// in this package, which is checkable by reading one file.
	t.Run("every wrong password yields the same error", func(t *testing.T) {
		t.Parallel()

		ctx, _ := observedCtx(t)
		svc := newService(password)

		sameLen := strings.Repeat("x", len(password))
		require.Len(t, sameLen, len(password))

		for _, wrong := range []string{sameLen, "short", password + "!", ""} {
			claims, err := svc.Authenticate(ctx, wrong)
			require.Nil(t, claims)
			require.ErrorIs(t, err, apperr.ErrInvalidCredentials)
		}
	})

	t.Run("an empty resolved password never authenticates", func(t *testing.T) {
		t.Parallel()

		// Defense in depth: ResolvePassword never returns empty, but if it ever
		// did, an empty credential must not become a skeleton key.
		ctx, _ := observedCtx(t)
		svc := newService("")

		claims, err := svc.Authenticate(ctx, "")
		require.Nil(t, claims)
		require.ErrorIs(t, err, apperr.ErrInvalidCredentials)
	})
}

// The password must not reach the log from the AUTHENTICATE path either — not
// on success, not on failure. TestResolvePassword covers only startup, so
// without this a leak added here would go unnoticed: verified by mutation
// (logging s.password on the mismatch branch leaves the resolve tests green and
// fails only this one).
func TestServiceAuthenticate_NeverLogsThePassword(t *testing.T) {
	t.Parallel()

	const password = "the-secret-break-glass-password"
	ctx, logs := observedCtx(t)
	svc := newService(password)

	_, err := svc.Authenticate(ctx, password)
	require.NoError(t, err)
	_, err = svc.Authenticate(ctx, "wrong-password-entirely")
	require.ErrorIs(t, err, apperr.ErrInvalidCredentials)

	require.NotContains(t, loggedText(logs), password,
		"the break-glass password must never be logged outside ResolvePassword")
}

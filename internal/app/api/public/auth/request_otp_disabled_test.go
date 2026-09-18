package auth

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

// Criterion 2: with email codes disabled, requesting one issues NOTHING.
//
// Note how that is observed, because it cannot be observed from the response.
// This endpoint answers every outcome identically -- same 202, same nonce shape,
// same floor -- and that uniformity is deliberate: it is what stops the endpoint
// reporting whether an address belongs to anyone. A refusal that looked
// different would be a worse leak than the one being prevented.
//
// So the assertion is on the SIDE EFFECT: no credential row appears for the
// address. A caller cannot tell; the database can.
func TestRequestOTP_DisabledMethodIssuesNoCode(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t).WithAuthSettings(stubAuthSettings{
		enabled: map[entity.AuthMethodName]bool{entity.AuthMethodNameEmailOTP: false},
	})

	user := makeOTPUser(t, impl)

	resp := doRequestOTP(t, impl, `{"email":"`+user+`"}`)
	require.Equal(t, http.StatusAccepted, resp.status,
		"the refusal must be indistinguishable from an issued code")

	require.Zero(t, otpCodeCount(ctx, t, user),
		"no code may be issued while the method is disabled")
}

// With the method enabled the same call DOES issue, which is what makes the
// test above about the gate rather than about a broken fixture.
func TestRequestOTP_EnabledMethodStillIssues(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t).WithAuthSettings(stubAuthSettings{
		enabled: map[entity.AuthMethodName]bool{entity.AuthMethodNameEmailOTP: true},
	})

	user := makeOTPUser(t, impl)

	resp := doRequestOTP(t, impl, `{"email":"`+user+`"}`)
	require.Equal(t, http.StatusAccepted, resp.status)

	require.NotZero(t, otpCodeCount(ctx, t, user),
		"an enabled method must still issue")
}

// otpCodeCount counts the one-time codes held for an address.
//
// Read straight from the table rather than through the service, so the
// assertion does not depend on the very code path under test -- and so no
// accessor has to be added to a service purely to let a test look inside it.
func otpCodeCount(ctx context.Context, t *testing.T, email string) int {
	t.Helper()

	var n int
	require.NoError(t, db.GetContext(ctx, &n, `
		SELECT COUNT(*)
		  FROM auth_credentials c
		  JOIN users u ON u.id = c.user_id
		 WHERE u.email = $1 AND c.kind = 'otp'`, email))

	return n
}

// A handler with NO flag source issues nothing either.
//
// Not a repeat of the disabled case: that one asserts the gate reads the flag,
// this one asserts what the gate does when there is nothing to read. Both must
// refuse, and the reasoning differs -- bootstrap wires the settings service into
// every binary serving this route, so a nil is a dropped wiring line rather than
// an instance configured that way.
//
// It is worth its own test because the nil branch is the one a refactor deletes
// by accident. Answering permissively there would mean a single missing line in
// bootstrap silently re-opens every method an admin closed, with nothing in the
// response to show for it -- this endpoint answers identically either way.
func TestRequestOTP_NoFlagSourceIssuesNoCode(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t).WithAuthSettings(nil)

	user := makeOTPUser(t, impl)

	resp := doRequestOTP(t, impl, `{"email":"`+user+`"}`)
	require.Equal(t, http.StatusAccepted, resp.status,
		"an unwired handler must still answer like every other outcome")

	require.Zero(t, otpCodeCount(ctx, t, user),
		"no code may be issued when nothing answers whether the method is offered")
}

// An unreadable flag issues nothing either: the gate fails CLOSED.
//
// The third of the three ways this gate can decline, and the one that is not
// about configuration at all -- the admin enabled nothing and disabled nothing,
// the table simply did not answer.
//
// Failing closed here is the opposite of what the listing on the same endpoint
// does with the same error, and the asymmetry is deliberate: the listing only
// decides which buttons to draw, so showing fewer is the cheap direction to be
// wrong in, while issuing a code for a method that may be off is handing out a
// credential every time the database hiccups.
func TestRequestOTP_UnreadableFlagIssuesNoCode(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t).WithAuthSettings(stubAuthSettings{unreadable: true})

	user := makeOTPUser(t, impl)

	resp := doRequestOTP(t, impl, `{"email":"`+user+`"}`)
	require.Equal(t, http.StatusAccepted, resp.status,
		"the refusal must be indistinguishable from an issued code")

	require.Zero(t, otpCodeCount(ctx, t, user),
		"no code may be issued while the flag cannot be read")
}

package auth

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

// Criterion 1: a disabled method is not listed, and the other one still is.
//
// The ids are asserted as LITERALS and the flags are set through the typed
// closed set, so the test crosses the two independent spellings of the same two
// strings -- which is the only thing that makes the wire contract and the table
// provably the same set.
func TestListAuthMethods_OmitsADisabledBuiltIn(t *testing.T) {
	t.Parallel()

	impl := initImpl(t).
		WithAuthSettings(stubAuthSettings{enabled: map[entity.AuthMethodName]bool{
			entity.AuthMethodNameEmailPassword: true,
			entity.AuthMethodNameEmailOTP:      false,
		}})

	resp := doListAuthMethods(t, impl)

	require.Equal(t, []string{"email_password"}, idsOf(t, resp.body))
}

func TestListAuthMethods_OmitsBothWhenBothAreDisabled(t *testing.T) {
	t.Parallel()

	impl := initImpl(t).
		WithAuthSettings(stubAuthSettings{enabled: map[entity.AuthMethodName]bool{
			entity.AuthMethodNameEmailPassword: false,
			entity.AuthMethodNameEmailOTP:      false,
		}})

	resp := doListAuthMethods(t, impl)

	require.Empty(t, idsOf(t, resp.body),
		"an instance with everything disabled reports no way in rather than erroring")
}

// An unreadable table DEGRADES the listing rather than failing it: the built-ins
// drop out and the response still renders, providers included.
//
// This endpoint grants nothing, so hiding buttons is the cheap direction to be
// wrong in -- and a caller who guesses a hidden method still meets the sign-in
// gate, which fails closed. Answering 500 instead would leave the login page
// unable to render at all, which is the expensive direction.
//
// Note the granularity: the flags are read in ONE query, so a failure is "the
// table did not answer", not "this method's flag did not answer". Both built-ins
// drop together because that is what actually happened -- reporting one of them
// as offered would be inventing an answer the database never gave.
func TestListAuthMethods_DegradesWhenTheFlagsAreUnreadable(t *testing.T) {
	t.Parallel()

	// No enabled map: List fails before reading it, and setting one would imply
	// this test checks that email_password survives while email_otp fails. It
	// does not -- both drop, because the read failed for both.
	impl := initImpl(t).
		WithAuthSettings(stubAuthSettings{unreadable: true})

	resp := doListAuthMethods(t, impl)

	require.Equal(t, 200, resp.status, "a broken read must not take the login page down")
	require.Empty(t, idsOf(t, resp.body))
}

// With both flags enabled the response is exactly what it was before the table
// existed.
//
// Not what the migration produces -- it seeds email_otp OFF (§5.1) -- but what
// the listing does when told both are on: it still prepends the same two
// built-ins, in the same order, with the same ids. That is what keeps the table
// a filter over the old behavior rather than a rewrite of it.
func TestListAuthMethods_UnchangedWhenBothEnabled(t *testing.T) {
	t.Parallel()

	withSettings := initImpl(t).
		WithAuthSettings(stubAuthSettings{enabled: map[entity.AuthMethodName]bool{
			entity.AuthMethodNameEmailPassword: true,
			entity.AuthMethodNameEmailOTP:      true,
		}})
	// A binary wired without the flags at all -- the pre-change shape.
	withoutSettings := initImpl(t)

	require.Equal(t,
		doListAuthMethods(t, withoutSettings).body,
		doListAuthMethods(t, withSettings).body,
	)
}

// bootstrap must never appear, whatever the flags say. It is the break-glass
// credential, and listing it on a public unauthenticated endpoint would answer
// "does this deployment have an emergency entrance" in one GET.
func TestListAuthMethods_NeverListsBootstrap(t *testing.T) {
	t.Parallel()

	impl := initImpl(t).
		WithAuthSettings(stubAuthSettings{enabled: map[entity.AuthMethodName]bool{
			entity.AuthMethodNameEmailPassword: false,
			entity.AuthMethodNameEmailOTP:      false,
		}})

	require.NotContains(t, doListAuthMethods(t, impl).body, "bootstrap")
}

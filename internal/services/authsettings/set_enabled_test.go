package authsettings_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// Criterion 12: an unknown method is refused, and nothing is created for it.
//
// The second half is the part worth asserting: a service that upserted instead
// of looking up would answer 200 here and leave a row the closed set does not
// contain, which nothing downstream would ever read.
//
// Note what this does NOT prove. Removing the IsValid() check from SetEnabled
// leaves this test green, because findMethod refuses the same input one layer
// down, inside the transaction. The two are not redundant -- IsValid() refuses
// before a transaction is opened and every row is locked -- but nothing here
// distinguishes them, and the obvious way to try (counting transactions through
// pg_stat_database) was written, found green under mutation, and deleted rather
// than kept as decoration.
func TestSetEnabled_UnknownMethodIsRefusedAndCreatesNothing(t *testing.T) {
	svc, _ := initService(t)
	ctx := context.Background()

	before, err := svc.List(ctx)
	require.NoError(t, err)

	_, err = svc.SetEnabled(ctx, &entity.SetAuthMethodEnabledCmd{
		Method:  entity.AuthMethodName("sms_otp"),
		Enabled: true,
		Actor:   testActor(),
	})
	require.ErrorIs(t, err, apperr.ErrAuthMethodNotFound)

	after, err := svc.List(ctx)
	require.NoError(t, err)
	require.Len(t, after, len(before), "an unknown method must not add a row")
}

// Criterion 13: setting a flag to the value it already holds succeeds and
// converges.
//
// This is what SetEnabled buys over a Toggle: a retried request lands on the
// state the caller asked for instead of flipping back to where it started.
func TestSetEnabled_IsIdempotent(t *testing.T) {
	svc, _ := initService(t)
	ctx := context.Background()

	first, err := svc.SetEnabled(ctx, &entity.SetAuthMethodEnabledCmd{
		Method:  entity.AuthMethodNameEmailOTP,
		Enabled: false,
		Actor:   testActor(),
	})
	require.NoError(t, err)
	require.False(t, first.Enabled)

	second, err := svc.SetEnabled(ctx, &entity.SetAuthMethodEnabledCmd{
		Method:  entity.AuthMethodNameEmailOTP,
		Enabled: false,
		Actor:   testActor(),
	})
	require.NoError(t, err)
	require.False(t, second.Enabled, "a repeated request must converge, not flip")
	require.False(t, enabledOf(t, svc, entity.AuthMethodNameEmailOTP))
}

func TestSetEnabled_WritesTheFlagAndTheActor(t *testing.T) {
	svc, _ := initService(t)
	ctx := context.Background()

	// Both on: this test disables one and asserts the OTHER is untouched, so it
	// needs two enabled rows to have something to leave alone -- and with only
	// one enabled the guard would refuse the write before it ever got there.
	enableAll(t, svc)

	actor := testActor()
	updated, err := svc.SetEnabled(ctx, &entity.SetAuthMethodEnabledCmd{
		Method:  entity.AuthMethodNameEmailPassword,
		Enabled: false,
		Actor:   actor,
	})
	require.NoError(t, err)
	require.False(t, updated.Enabled)
	require.Equal(t, &actor.ID, updated.UpdatedByUserID)

	require.False(t, enabledOf(t, svc, entity.AuthMethodNameEmailPassword))
	require.True(t, enabledOf(t, svc, entity.AuthMethodNameEmailOTP),
		"the other method must be untouched")
}

// Enabled answers per method rather than for the table as a whole. Worth its
// own test because every gate depends on it, and a lookup that ignored its
// argument would pass every other test in this file.
func TestEnabled_AnswersPerMethod(t *testing.T) {
	svc, _ := initService(t)
	ctx := context.Background()

	_, err := svc.SetEnabled(ctx, &entity.SetAuthMethodEnabledCmd{
		Method:  entity.AuthMethodNameEmailOTP,
		Enabled: false,
		Actor:   testActor(),
	})
	require.NoError(t, err)

	require.False(t, enabledOf(t, svc, entity.AuthMethodNameEmailOTP))
	require.True(t, enabledOf(t, svc, entity.AuthMethodNameEmailPassword))
}

// A method outside the closed set has no answer, and Enabled must say so rather
// than reporting false -- the gates read this, and "false" would silently
// disable a method somebody misspelled.
func TestEnabled_UnknownMethodIsAFault(t *testing.T) {
	svc, _ := initService(t)

	_, err := svc.Enabled(context.Background(), entity.AuthMethodName("sms_otp"))
	require.ErrorIs(t, err, apperr.ErrAuthMethodNotFound)
}

// A missing actor is refused rather than tolerated.
//
// Not reachable today -- every caller is an authenticated admin -- but the
// alternative to checking is a nil dereference inside a log line on the refusal
// path, which would turn a wiring mistake into a panic on exactly the request
// that was trying to prevent a lockout. An obligatory invariant should fail
// loudly, not degrade.
func TestSetEnabled_RequiresAnActor(t *testing.T) {
	svc, _ := initService(t)

	_, err := svc.SetEnabled(context.Background(), &entity.SetAuthMethodEnabledCmd{
		Method:  entity.AuthMethodNameEmailOTP,
		Enabled: false,
		Actor:   nil,
	})
	require.ErrorIs(t, err, apperr.ErrValidation)
}

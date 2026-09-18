package authsettings_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
)

// Criterion 10: a toggle produces exactly one audit row naming the method and
// the new state.
func TestSetEnabled_AuditsTheChange(t *testing.T) {
	svc, spy := initService(t)

	actor := testActor()
	_, err := svc.SetEnabled(context.Background(), &entity.SetAuthMethodEnabledCmd{
		Method:  entity.AuthMethodNameEmailOTP,
		Enabled: false,
		Actor:   actor,
	})
	require.NoError(t, err)

	require.Len(t, spy.Actions(), 1, "one change, one row")

	toggled, ok := spy.Actions()[0].(audit.AuthMethodToggled)
	require.True(t, ok, "expected an AuthMethodToggled action")
	require.Equal(t, entity.AuthMethodNameEmailOTP, toggled.Method)
	require.False(t, toggled.Enabled)
	require.Equal(t, actor.ID, toggled.Actor.ID)
}

// An unchanged write still audits. The admin performed the action, and a trail
// that silently drops no-ops leaves an operator unable to tell "nobody tried"
// from "somebody tried twice".
//
// This is the flip side of SetEnabled's idempotence: the STATE converges, the
// RECORD does not, and both halves are intended.
func TestSetEnabled_AuditsEvenWhenNothingChanged(t *testing.T) {
	svc, spy := initService(t)
	ctx := context.Background()

	for range 2 {
		_, err := svc.SetEnabled(ctx, &entity.SetAuthMethodEnabledCmd{
			Method:  entity.AuthMethodNameEmailOTP,
			Enabled: false,
			Actor:   testActor(),
		})
		require.NoError(t, err)
	}

	require.Len(t, spy.Actions(), 2, "two requests, two rows, one final state")
}

// A refused change must leave no audit row: the publish happens after the
// transaction commits, so a rejected request never reaches it.
func TestSetEnabled_RefusedChangeIsNotAudited(t *testing.T) {
	svc, spy := initService(t)

	_, err := svc.SetEnabled(context.Background(), &entity.SetAuthMethodEnabledCmd{
		Method:  entity.AuthMethodName("sms_otp"),
		Enabled: true,
		Actor:   testActor(),
	})
	require.Error(t, err)
	require.Empty(t, spy.Actions(), "a refused change is not a change")
}

package authsettings

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// The migration seeds every built-in, and the rest of the system reads that as
// a promise: a missing row is treated as a fault rather than defaulted, so if
// the seed were ever incomplete the fault would surface at sign-in instead of
// here.
func TestList_ReturnsEverySeededBuiltIn(t *testing.T) {
	settings, err := store.List(context.Background())
	require.NoError(t, err)

	got := make(map[entity.AuthMethodName]bool, len(settings))
	for _, s := range settings {
		got[s.Method] = true
	}

	for _, want := range entity.AllAuthMethodNames() {
		require.Truef(t, got[want], "no seeded row for %q", want)
	}
}

func TestUpdate_WritesFlagAndAuthorship(t *testing.T) {
	before := get(t, entity.AuthMethodNameEmailOTP)
	restoreEnabled(t, entity.AuthMethodNameEmailOTP, before.Enabled)

	actor := uuid.New()
	before.Enabled = !before.Enabled
	before.UpdatedByUserID = &actor

	updated, err := store.Update(context.Background(), before)
	require.NoError(t, err)
	require.Equal(t, before.Enabled, updated.Enabled)
	require.Equal(t, &actor, updated.UpdatedByUserID)

	// Read back through a second query rather than trusting RETURNING: the
	// point of the test is that the row changed, not that the statement echoed
	// its own input.
	reread := get(t, entity.AuthMethodNameEmailOTP)
	require.Equal(t, before.Enabled, reread.Enabled)
	require.Equal(t, &actor, reread.UpdatedByUserID)
}

// updated_at is stamped by the store, not the caller, so a caller that forgets
// cannot leave a row looking untouched after a change.
func TestUpdate_StampsUpdatedAt(t *testing.T) {
	before := get(t, entity.AuthMethodNameEmailPassword)
	restoreEnabled(t, entity.AuthMethodNameEmailPassword, before.Enabled)

	stale := before.UpdatedAt
	before.UpdatedAt = stale.AddDate(-1, 0, 0)
	before.Enabled = !before.Enabled

	updated, err := store.Update(context.Background(), before)
	require.NoError(t, err)
	require.Truef(t, updated.UpdatedAt.After(stale),
		"updated_at must advance: was %s, got %s", stale, updated.UpdatedAt)
}

// GetByMethod reads the row its argument names, not merely a row.
//
// Both methods are asserted, which is what distinguishes a working lookup from
// one that ignores its argument: with only one method checked, a query hard-wired
// to that method passes. The flags are read from List first rather than assumed,
// so the test does not depend on what the migration seeded.
func TestGetByMethod_ReadsTheNamedRow(t *testing.T) {
	ctx := context.Background()

	all, err := store.List(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, all)

	for _, want := range all {
		got, getErr := store.GetByMethod(ctx, want.Method)
		require.NoError(t, getErr)

		require.Equal(t, want.Method, got.Method)
		require.Equal(t, want.ID, got.ID)
		require.Equal(t, want.Enabled, got.Enabled)
	}
}

// A method with no row is ErrAuthMethodNotFound, carrying the name.
//
// Not-found is a domain answer rather than a raw qrm error because both causes
// -- a name outside the closed set, and a seeded row that went missing -- need
// the caller to refuse rather than guess, and neither has a safe default.
func TestGetByMethod_MissingRowIsADomainError(t *testing.T) {
	_, err := store.GetByMethod(context.Background(), entity.AuthMethodName("sms_otp"))

	require.ErrorIs(t, err, apperr.ErrAuthMethodNotFound)
	require.ErrorContains(t, err, "sms_otp")
}

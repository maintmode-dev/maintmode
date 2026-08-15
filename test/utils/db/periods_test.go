package testdbutils

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

// TestIsolatedPeriodBounds_AlwaysInTheFuture guards the invariant the helper's
// whole purpose depends on: an isolated window must be far in the FUTURE.
//
// It is pinned because the obvious implementation silently breaks it. Offsetting
// by `time.Duration(slot) * stride` overflows — a Duration is an int64 of
// nanoseconds, so at a 30-day stride it wraps past slot 3558 — and since
// testNameOffset returns 1024..5119, roughly 30% of test names produced a
// negative offset and landed their window about a century in the past.
//
// The failure was not a loud one. A past-dated maintenance is a legitimate
// target for the auto-cancel sweep, so the fixture would be canceled as
// never-started and its owning test would fail with a status it never set —
// intermittently, and in a different package from the one that looked guilty.
func TestIsolatedPeriodBounds_AlwaysInTheFuture(t *testing.T) {
	now := xtime.UTCNow()

	// Names chosen because they hash into the overflow range and did land in
	// 1836 / 1935 / 1914 before the fix.
	for _, name := range []string{
		"TestApprove/ok",
		"TestApprove/change_maint_revision",
		"TestApprove/non-admin_approver-eligible_role_stays_bound_to_the_assignment",
		"TestConflictResources_IndependentOfViewer",
		"TestSomethingElseEntirely",
	} {
		t.Run(name, func(t *testing.T) {
			start, end := IsolatedPeriodBounds(t)

			require.True(t, start.After(now),
				"window must be in the future, got start=%s (now=%s)", start, now)
			require.True(t, end.After(start),
				"end must follow start, got start=%s end=%s", start, end)
		})
	}
}

// TestIsolatedPeriodBounds_SlotsDoNotOverlap checks the other half of the
// contract: consecutive windows must not touch, even after a caller widens its
// own by the few hours the maintenance fixtures use.
func TestIsolatedPeriodBounds_SlotsDoNotOverlap(t *testing.T) {
	const widening = 12 // hours a caller may stretch its window in either direction

	_, firstEnd := IsolatedPeriodBounds(t)
	secondStart, _ := IsolatedPeriodBounds(t)

	require.True(t, secondStart.After(firstEnd),
		"consecutive windows must not overlap: first ends %s, second starts %s",
		firstEnd, secondStart)

	gap := secondStart.Sub(firstEnd).Hours()
	require.Greater(t, gap, float64(2*widening),
		"the gap between windows must survive both sides widening: got %.0fh", gap)
}

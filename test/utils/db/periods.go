package testdbutils

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

// isolatedPeriodSeq separates windows handed out within one process. It is only
// half of the isolation — see IsolatedPeriodBounds for the other half and for
// why one counter cannot be enough on its own.
var isolatedPeriodSeq atomic.Int64

// isolatedPeriodEpoch is the far-future anchor every isolated window hangs off.
//
// A century out is not superstition: it has to clear every window the suite
// builds from xtime.UTCNow(), and those reach hours to days ahead, with the
// audit and invitation fixtures reaching years BACK. Anchoring beyond all of
// them means a "now"-relative window can never wander into an isolated one.
const isolatedPeriodEpoch = 100

// isolatedPeriodStrideMonths is the gap between consecutive windows. It must
// exceed the span any single test builds on top of its window — the maintenance
// fixtures stretch a base period by a few hours in each direction — with room
// left over, so neighboring slots cannot touch even after a test widens its own.
//
// Counted in calendar months and applied with AddDate rather than as a
// time.Duration, and that is not a style choice. A Duration is an int64 of
// nanoseconds, so it tops out around 292 years: at a 30-day stride, slot 3559
// overflows. Slots run to 5119 (see testNameOffset), so roughly 30% of test
// names used to wrap to a NEGATIVE offset and land their "far future" window
// about a century in the PAST — where the auto-cancel sweep would then
// legitimately cancel the fixture as never-started, and the owning test would
// find its own maintenance canceled out from under it.
const isolatedPeriodStrideMonths = 1

// IsolatedPeriodBounds returns a maintenance window no other test can overlap,
// as the start/end pair the fixtures build their periods from.
//
// Conflict detection is deliberately blind to almost everything a test can
// uniquify. A maintenance whose scope is global conflicts with EVERY other
// maintenance in an overlapping time range, whatever resources either one names
// (see the impact-zone clause in the conflicted-maints query), so seeding fresh
// resource ids per test — which the fixtures already do — buys no protection at
// all against a global-scope row. Status and time range are the only other
// filters, and status is usually fixed by what the test is about.
//
// That leaves the time range as the one axis a test can actually claim, and the
// suite does not claim it today: packages across the repo build their windows
// from xtime.UTCNow() and run hours forward, so under `make tloc` — which runs
// `-count 2` with `-p 2` against ONE shared Postgres — several packages sit in
// overlapping windows at the same moment. A global-scope fixture from any of
// them lands inside another package's window and silently changes what that
// package's conflict queries return.
//
// The isolation is two-part because one part cannot do it:
//   - the process-wide counter separates windows inside a single test binary,
//     but every `-count 2` rerun and every concurrently-running package starts
//     its own counter at zero, so counters alone collide across processes;
//   - the per-test-name offset separates those processes, since two tests
//     racing on the shared database are by definition different tests.
//
// Callers get a window and may stretch it by a few hours in either direction,
// which is what the maintenance fixtures do; the stride leaves room for that.
func IsolatedPeriodBounds(t *testing.T) (start, end time.Time) {
	t.Helper()

	slot := isolatedPeriodSeq.Add(1) + testNameOffset(t)

	// AddDate for the slot offset too, not Add(slot * stride): the multiplication
	// overflows int64 nanoseconds past slot 3558 and silently flips the window
	// into the past. See isolatedPeriodStrideMonths.
	start = xtime.UTCNow().
		AddDate(isolatedPeriodEpoch, int(slot)*isolatedPeriodStrideMonths, 0)

	// Five hours is the span the maintenance tests conventionally use, and it
	// stays far inside one stride even after a caller widens it.
	return start, start.Add(5 * time.Hour)
}

// testNameOffset maps a test's name to a stable slot far from the counter's own
// range, so windows from concurrently-running packages cannot collide.
//
// The name is the only identifier available that is stable across a `-count 2`
// rerun yet differs between the tests racing on the shared database. Hashing it
// rather than registering slots by hand keeps the helper drop-in: a new test
// gets isolation without editing a central table that nobody would think to
// update.
//
// Collisions are possible in principle — this is a hash into a bounded space —
// but two tests collide only if their names hash together AND they run at the
// same moment AND one of them is global-scope. The alternative, a hand-kept
// registry, trades that residual risk for a file that goes stale silently.
func testNameOffset(t *testing.T) int64 {
	t.Helper()

	const (
		fnvOffset = 14695981039346656037
		fnvPrime  = 1099511628211
		// Leaves the low slots to the in-process counter and keeps the product
		// of slot and stride well inside a time.Duration.
		slotSpace  = 4096
		slotOffset = 1024
	)

	hash := uint64(fnvOffset)
	for _, b := range []byte(t.Name()) {
		hash ^= uint64(b)
		hash *= fnvPrime
	}

	return int64(hash%slotSpace) + slotOffset //nolint:gosec // bounded by slotSpace
}

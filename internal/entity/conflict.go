package entity

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

type Conflict struct {
	MaintenanceID uuid.UUID
	Title         string
	OverlapStart  time.Time
	OverlapEnd    time.Time
	Scope         MaintenanceScope
}

type ConflictWithResources struct {
	*Conflict
	Resources []uuid.UUID
	// KnownAtApproval reports whether the approver saw this neighboring
	// maintenance when they approved. Read-side only: it is filled by
	// MarkKnownAtApproval and deliberately lives on the wrapper rather than on
	// Conflict, so it stays out of ConflictFingerprint — which gates approve.
	KnownAtApproval bool
}

type ConflictsSnapshot struct {
	Conflicts []*ConflictWithResources
}

// ConflictFingerprintFor hashes the conflict set as the approve gate sees it:
// each neighbor contributes only the resources it SHARES with the maintenance
// being approved, not everything that neighbor happens to touch.
//
// The distinction matters because the read side and the gate want different
// things from the same field. A reader opening the card wants to know what a
// neighbor touches, so the view reports its full resource set. The gate answers a
// narrower question — did the thing the approver agreed to change? — and a
// neighbor picking up a resource we do not hold does not change it: the two
// maintenances still collide over exactly the same resources, in the same window.
// Hashing the full set there would bounce approvals for edits happening anywhere
// in the window, and a gate that cries wolf teaches people to retry blindly.
//
// This is the same rule MarkKnownAtApproval already applies — a neighbor whose
// resource set shifted is still the same neighbor — so the two mechanisms agree.
//
// ownResourceIDs is the approving maintenance's own resource set. Empty means a
// global-scope maintenance, which shares no resources with anyone; every overlap
// is then empty and the fingerprint rests on the neighbors and their windows,
// which is correct — a global-scope maintenance conflicts by time, not by
// resource.
func ConflictFingerprintFor(conflicts []*ConflictWithResources, ownResourceIDs []uuid.UUID) string {
	own := make(map[uuid.UUID]struct{}, len(ownResourceIDs))
	for _, id := range ownResourceIDs {
		own[id] = struct{}{}
	}

	// Project onto the overlap in fresh slices. The inputs are the live read-path
	// result and the client's snapshot, and both outlive this call — the snapshot
	// is persisted verbatim right after the gate passes — so neither may be
	// mutated here.
	projected := make([]*ConflictWithResources, 0, len(conflicts))
	for _, c := range conflicts {
		if c == nil {
			continue
		}

		shared := make([]uuid.UUID, 0, len(c.Resources))
		for _, id := range c.Resources {
			if _, ok := own[id]; ok {
				shared = append(shared, id)
			}
		}

		projected = append(projected, &ConflictWithResources{
			Conflict:  c.Conflict,
			Resources: shared,
		})
	}

	return ConflictFingerprint(projected)
}

// ConflictFingerprint hashes a conflict set exactly as given. Callers gating an
// approval want ConflictFingerprintFor, which first narrows each neighbor to the
// resources it shares with the maintenance being approved.
func ConflictFingerprint(conflicts []*ConflictWithResources) string {
	if len(conflicts) == 0 {
		sum := sha256.Sum256([]byte("EMPTY_CONFLICT_SET"))
		return hex.EncodeToString(sum[:])
	}

	slices.SortFunc(conflicts, func(c1, c2 *ConflictWithResources) int {
		if c1.MaintenanceID != c2.MaintenanceID {
			return xuuid.Compare(c1.MaintenanceID, c2.MaintenanceID)
		}

		if !c1.OverlapStart.Equal(c2.OverlapStart) {
			return c1.OverlapStart.Compare(c2.OverlapStart)
		}

		if !c1.OverlapEnd.Equal(c2.OverlapEnd) {
			return c1.OverlapEnd.Compare(c2.OverlapEnd)
		}

		return strings.Compare(string(c1.Scope), string(c2.Scope))
	})

	for _, conflict := range conflicts {
		SortResources(conflict.Resources)
	}

	buf := make([]byte, 0, len(conflicts)*200)
	for _, c := range conflicts {
		if c == nil {
			continue
		}

		// maintenance_id
		buf = append(buf, "maintenance_id="...)
		buf = append(buf, c.MaintenanceID[:]...)

		// scope
		buf = append(buf, "scope="...)
		buf = append(buf, c.Scope...)

		// overlap_start
		buf = append(buf, "overlap_start="...)
		buf = strconv.AppendInt(buf, c.OverlapStart.UTC().Truncate(time.Second).Unix(), 10)

		// overlap_end
		buf = append(buf, "overlap_end="...)
		buf = strconv.AppendInt(buf, c.OverlapEnd.UTC().Truncate(time.Second).Unix(), 10)

		// resources
		buf = append(buf, "resources="...)
		buf = append(buf, conflictResourcesFingerprint(c.Resources)...)
	}

	sum := sha256.Sum256(buf)
	return hex.EncodeToString(sum[:])
}

func conflictResourcesFingerprint(resources []uuid.UUID) []byte {
	buf := make([]byte, 0, len(resources)*20)
	for _, id := range resources {
		buf = append(buf, "id="...)
		buf = append(buf, id[:]...)
	}

	return buf
}

func SortResources(resources []uuid.UUID) {
	slices.SortFunc(resources, xuuid.Compare)
}

// MarkKnownAtApproval flags every live conflict the approver had already seen,
// by matching it against the snapshot frozen at approval time.
//
// The match is on MaintenanceID alone. The flag answers "did the approver
// consider this neighbor", not "is every attribute still byte-identical", so a
// neighbor that merely shifted in time or changed its resource set stays known
// — false alarms would train the on-call to ignore the highlight, which is the
// one failure this feature cannot afford.
//
// Do not "strengthen" the match by comparing resources: a neighbor whose
// resource set moved is precisely one the approver did consider, so flagging it
// as unreviewed produces exactly the false alarm described above.
// A conflict without its embedded Conflict is an invariant violation, not an
// input to tolerate: every producer allocates it. Skipping such an element would
// silently drop a conflict from a screen whose purpose is to warn, and
// SortConflicts would dereference it anyway — so let it fail loudly instead.
func MarkKnownAtApproval(live, snapshot []*ConflictWithResources) {
	if len(live) == 0 || len(snapshot) == 0 {
		return
	}

	known := make(map[uuid.UUID]struct{}, len(snapshot))
	for _, conflict := range snapshot {
		known[conflict.MaintenanceID] = struct{}{}
	}

	for _, conflict := range live {
		_, ok := known[conflict.MaintenanceID]
		conflict.KnownAtApproval = ok
	}
}

// SortConflicts imposes the response order: conflicts nobody reviewed first,
// then by overlap start, then by maintenance id.
//
// The order is defined here rather than in SQL because neither source provides
// one — ConflictedMaints has no ORDER BY, and the snapshot mapper iterates a Go
// map, whose iteration order is randomized per call. Sorting the merged list
// makes the contract independent of both.
func SortConflicts(conflicts []*ConflictWithResources) {
	slices.SortFunc(conflicts, func(c1, c2 *ConflictWithResources) int {
		if c1.KnownAtApproval != c2.KnownAtApproval {
			// false (unreviewed) sorts before true.
			if c1.KnownAtApproval {
				return 1
			}
			return -1
		}

		if !c1.OverlapStart.Equal(c2.OverlapStart) {
			return c1.OverlapStart.Compare(c2.OverlapStart)
		}

		return xuuid.Compare(c1.MaintenanceID, c2.MaintenanceID)
	})
}

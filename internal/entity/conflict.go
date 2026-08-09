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
// Nothing else in the snapshot can discriminate anyway: scope is not stored (it
// is resolved by a live join, so both sides always agree), and resources are an
// intersection with the querying maintenance's own set, absent entirely from
// snapshots the UI writes.
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

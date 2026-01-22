package entity

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

type Conflict struct {
	MaintenanceID uuid.UUID
	OverlapStart  time.Time
	OverlapEnd    time.Time
	Scope         MaintenanceScope
}

type ConflictWithResources struct {
	*Conflict
	Resources []*Resource
}

type ConflictsSnapshot struct {
	Fingerprint string
	Conflicts   []*ConflictWithResources
}

func ConflictFingerprint(conflicts []*ConflictWithResources) string {
	if len(conflicts) == 0 {
		sum := sha256.Sum256([]byte("EMPTY_CONFLICT_SET"))
		return hex.EncodeToString(sum[:])
	}

	sort.Slice(conflicts, func(i, j int) bool {
		c1, c2 := conflicts[i], conflicts[j]

		if c1.MaintenanceID != c2.MaintenanceID {
			return xuuid.CompareBool(c1.MaintenanceID, c2.MaintenanceID)
		}
		if c1.OverlapStart.Equal(c2.OverlapStart) {
			return c1.OverlapStart.Equal(c2.OverlapStart)
		}

		if c1.OverlapEnd.Equal(c2.OverlapEnd) {
			return c1.OverlapEnd.Equal(c2.OverlapEnd)
		}

		return c1.Scope < c2.Scope
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

func conflictResourcesFingerprint(resources []*Resource) []byte {
	buf := make([]byte, 0, len(resources)*50)
	for _, r := range resources {
		buf = append(buf, "id="...)
		buf = append(buf, r.ID[:]...)
		buf = append(buf, "type="...)
		buf = append(buf, r.Type...)
	}

	return buf
}

func SortResources(resources []*Resource) {
	sort.Slice(resources, func(i, j int) bool {
		r1, r2 := resources[i], resources[j]

		if r1.ID != r2.ID {
			return xuuid.CompareBool(r1.ID, r2.ID)
		}
		return r1.Type < r2.Type
	})
}

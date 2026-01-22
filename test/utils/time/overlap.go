package testtimeutils

import (
	"time"

	"github.com/ruko1202/maintmode/internal/entity"
)

func OverlapStart(maintPlannedPeriod, conflictedMaintPlannedPeriod entity.Period) time.Time {
	return time.UnixMicro(
		max(maintPlannedPeriod.Start.UnixMicro(), conflictedMaintPlannedPeriod.Start.UnixMicro()),
	).UTC()
}

func OverlapEnd(maintPlannedPeriod, conflictedMaintPlannedPeriod entity.Period) time.Time {
	return time.UnixMicro(
		min(maintPlannedPeriod.End.UnixMicro(), conflictedMaintPlannedPeriod.End.UnixMicro()),
	).UTC()
}

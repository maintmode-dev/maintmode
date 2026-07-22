package invitationrotateprocessor

import (
	"fmt"
	"time"
)

// invitationRotateExternalID derives the deterministic, day-bucketed external id
// for an invitation.rotate task. Truncating to the day makes every replica that
// ticks within the same day produce the same id, so the goque (type,
// external_id) unique constraint dedupes them to a single enqueued task per day.
//
// The bucket is computed from wall-clock fire time, so replicas with clock skew
// straddling a day boundary can land in adjacent buckets and enqueue twice. That
// is harmless: rotation is idempotent (a second run on the same boundary finds
// no pending rows left past it) and bounded.
func invitationRotateExternalID(now time.Time) string {
	return fmt.Sprintf("invitation-rotate-%s", now.UTC().Format("2006-01-02"))
}

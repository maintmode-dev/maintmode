package autocancelprocessor

import (
	"fmt"
	"time"
)

// autoCancelExternalID derives the deterministic, minute-bucketed external id for
// a maint.auto_cancel task. Truncating to the minute makes every replica that
// ticks within the same minute produce the same id, so the goque (type,
// external_id) unique constraint dedupes them to a single enqueued task.
//
// The bucket is computed from wall-clock fire time, so replicas with clock skew
// straddling a minute boundary can land in adjacent buckets and enqueue twice in
// one minute. That is harmless: each sweep is idempotent and bounded, so the
// guarantee is at-most-twice rather than strictly at-most-once under skew.
func autoCancelExternalID(now time.Time) string {
	return fmt.Sprintf("maint-auto-cancel-%d", now.Truncate(time.Minute).Unix())
}

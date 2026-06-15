package auditpruneprocessor

import (
	"fmt"
	"time"
)

// auditPruneExternalID derives the deterministic, day-bucketed external id for an
// audit.prune task. Truncating to the day makes every replica that ticks within
// the same day produce the same id, so the goque (type, external_id) unique
// constraint dedupes them to a single enqueued task per day. Retention is a
// daily-granularity concern, so one prune per day is the intended cadence even if
// the cron schedule fires more often.
//
// The bucket is computed from wall-clock fire time, so replicas with clock skew
// straddling a day boundary can land in adjacent buckets and enqueue twice. That
// is harmless: the prune is idempotent (a second run on the same cutoff finds
// nothing left to delete) and bounded.
func auditPruneExternalID(now time.Time) string {
	return fmt.Sprintf("audit-prune-%s", now.UTC().Format("2006-01-02"))
}

package refreshtokenpruneprocessor

import (
	"fmt"
	"time"
)

// refreshTokenPruneExternalID derives the deterministic, day-bucketed external
// id for a refresh_token.prune task. Every replica that ticks within the same
// UTC day produces the same id, so the goque (type, external_id) unique
// constraint dedupes them to one enqueued task per day.
//
// Replicas with clock skew straddling midnight can land in adjacent buckets and
// enqueue twice. That is harmless: the prune is idempotent and bounded.
func refreshTokenPruneExternalID(now time.Time) string {
	return fmt.Sprintf("refresh-token-prune-%s", now.UTC().Format("2006-01-02"))
}

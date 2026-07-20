package licenseheartbeatprocessor

import (
	"fmt"
	"time"
)

// licenseHeartbeatExternalID derives the deterministic, minute-bucketed
// external id for a license.heartbeat task. Truncating to the minute makes
// every replica that ticks within the same minute produce the same id, so the
// goque (type, external_id) unique constraint dedupes them to a single
// enqueued heartbeat per minute — the contract cadence (~60s).
//
// Clock skew across a minute boundary can enqueue twice; that is harmless:
// the heartbeat is idempotent on the Console side (the report is a snapshot,
// the response overwrites the same cache row).
func licenseHeartbeatExternalID(now time.Time) string {
	return fmt.Sprintf("license-heartbeat-%s", now.UTC().Format("2006-01-02T15:04"))
}

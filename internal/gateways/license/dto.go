package license

import "time"

// Wire DTOs of the heartbeat contract. The per-role report is keyed by role
// with fixed keys — Console derives read-write/read-only groupings itself
// (read-write = admin+reviewer+editor, read-only = guest).
type heartbeatSeatBucket struct {
	Active  int64 `json:"active"`
	Pending int64 `json:"pending"`
}

type heartbeatPerRole struct {
	Admin    heartbeatSeatBucket `json:"admin"`
	Reviewer heartbeatSeatBucket `json:"reviewer"`
	Editor   heartbeatSeatBucket `json:"editor"`
	Guest    heartbeatSeatBucket `json:"guest"`
}

type heartbeatRequest struct {
	// PerRole is the disjoint per-role seat report; SeatsUsed is the
	// cap-relevant total: active admin+reviewer+editor. Guests are read-only
	// and excluded — Console compares SeatsUsed against seats_purchased and
	// derives any other grouping from per_role itself.
	PerRole   heartbeatPerRole `json:"per_role"`
	SeatsUsed int64            `json:"seats_used"`

	LastActivityAt *time.Time `json:"last_activity_at"`
	Version        string     `json:"version"`
}

type heartbeatResponse struct {
	Status         string `json:"status"`
	SeatsPurchased int64  `json:"seats_purchased"`
}

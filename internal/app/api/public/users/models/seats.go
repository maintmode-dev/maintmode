package apimodels

// SeatsResponse is the licensed-seat state an admin sees on the users page,
// before an invite runs into the cap.
//
// The three counters are always meaningful, unlimited or not: they describe the
// instance honestly, an unlimited one simply has no ceiling to compare against.
// SeatsOccupied is handed over ready-made rather than left to the client to add
// up, because it is the exact number the seat guard rejects an invite on — a
// client-side sum would drift the moment the rule about what holds a seat
// changes.
type SeatsResponse struct {
	// SeatsPurchased is the licensed ceiling, null when there is none.
	SeatsPurchased *int64 `json:"seats_purchased" example:"5"`
	// SeatsUsed counts active users holding a seat role.
	SeatsUsed int64 `json:"seats_used" example:"4"`
	// SeatsPending counts live invitations to a seat role. They hold a seat from
	// the moment they are sent, and are released by revoking or expiring them.
	SeatsPending int64 `json:"seats_pending" example:"1"`
	// SeatsOccupied is SeatsUsed plus SeatsPending — the number the seat cap is
	// checked against.
	SeatsOccupied int64 `json:"seats_occupied" example:"5"`
	// Unlimited is true on a self-hosted instance, or before a ceiling has been
	// reported. SeatsPurchased is null exactly when this is true.
	Unlimited bool `json:"unlimited" example:"false"`
}

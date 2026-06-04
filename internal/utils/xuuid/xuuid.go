// Package xuuid provides UUID generation utilities.
// It wraps the google/uuid library with fallback behavior.
package xuuid

import "github.com/google/uuid"

// New generates a new UUIDv7 or falls back to UUIDv4 if generation fails.
func New() uuid.UUID {
	u, err := uuid.NewV7()
	if err != nil {
		return uuid.New()
	}

	return u
}

func NewString() string {
	return New().String()
}

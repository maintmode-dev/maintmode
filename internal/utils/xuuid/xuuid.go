// Package xuuid provides UUID generation utilities.
// It wraps the google/uuid library with fallback behavior.
package xuuid

import "github.com/google/uuid"

// NewUUID generates a new UUIDv7 or falls back to UUIDv4 if generation fails.
func NewUUID() string {
	u, err := uuid.NewV7()
	if err != nil {
		return uuid.NewString()
	}

	return u.String()
}

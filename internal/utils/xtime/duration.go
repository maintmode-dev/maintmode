package xtime

import (
	"fmt"
	"time"
)

// ParseTimeout parses a Go duration string (e.g. "10s") from a Settings field. An
// empty string yields 0 so the transport applies its own default; an unparseable
// value is an error. Exported because both halves of a kind use it: Validate
// here, and the transport builder in services/transportresolver.
func ParseTimeout(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("timeout must be a duration (e.g. \"10s\"): %w", err)
	}
	return d, nil
}

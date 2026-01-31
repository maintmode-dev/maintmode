package uimodels

import (
	"fmt"
	"time"
)

// Date represents a calendar date without time.
// @Format date
// @Example 2026-01-15
type Date struct {
	time.Time
}

func (d *Date) UnmarshalText(text []byte) error {
	t, err := time.Parse(time.DateOnly, string(text))
	if err != nil {
		return fmt.Errorf("invalid date format, expected %v", time.DateOnly)
	}

	d.Time = t
	return nil
}

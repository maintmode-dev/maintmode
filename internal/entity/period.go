package entity

import (
	"fmt"
	"time"

	"github.com/samber/lo"
)

type Period struct {
	Start time.Time
	End   *time.Time
}

func NewPeriod(start, end time.Time) Period {
	return Period{
		Start: start,
		End:   &end,
	}
}

func NewOpenEndedPeriod(start time.Time) Period {
	return Period{
		Start: start,
		End:   nil,
	}
}

func (p Period) IsOpen() bool {
	return p.End == nil
}

func (p Period) Duration() time.Duration {
	return p.End.Sub(p.Start)
}

// String renders the period as a stable, human-readable UTC range. It uses
// RFC3339 in UTC (not time.Time's default String, which leaks the server
// location and a monotonic-clock suffix and would produce noisy/unstable diffs
// in the audit trail).
func (p Period) String() string {
	start := p.Start.UTC().Format(time.RFC3339)
	end := lo.Ternary(p.IsOpen(), "open ended", lo.FromPtr(p.End).UTC().Format(time.RFC3339))
	return fmt.Sprintf("%s - %s", start, end)
}

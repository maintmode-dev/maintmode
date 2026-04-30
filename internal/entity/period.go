package entity

import "time"

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

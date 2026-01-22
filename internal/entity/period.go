package entity

import "time"

type Period struct {
	Start *time.Time
	End   *time.Time
}

func NewPeriod(start, end time.Time) Period {
	return Period{
		Start: &start,
		End:   &end,
	}
}

func NewOpenEndedPeriod(start time.Time) Period {
	return Period{
		Start: &start,
		End:   nil,
	}
}

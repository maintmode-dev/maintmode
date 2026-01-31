package apimodels

import "time"

type Period struct {
	Start time.Time  `json:"start" format:"date-time" example:"2026-01-20T10:00:00Z"`
	End   *time.Time `json:"end,omitempty" format:"date-time" example:"2026-01-20T12:00:00Z"`
}

func (p Period) IsOpen() bool {
	return p.End == nil
}

package uicalendar

import (
	"github.com/ruko1202/maintmode/internal/services/calendar"
)

type Implementation struct {
	calendarSrv *calendar.Service
}

func New(calendarSrv *calendar.Service) *Implementation {
	return &Implementation{
		calendarSrv: calendarSrv,
	}
}

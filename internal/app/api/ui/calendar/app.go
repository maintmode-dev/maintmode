package uicalendar

import (
	"github.com/ruko1202/maintmode/internal/services/authz"
	"github.com/ruko1202/maintmode/internal/services/calendar"
)

type Implementation struct {
	calendarSrv *calendar.Service
	authorizer  *authz.CasbinAuthorizer
}

func New(calendarSrv *calendar.Service, authorizer *authz.CasbinAuthorizer) *Implementation {
	return &Implementation{
		calendarSrv: calendarSrv,
		authorizer:  authorizer,
	}
}

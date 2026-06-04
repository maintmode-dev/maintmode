package uicalendar

import (
	"github.com/ruko1202/maintmode/internal/services/authz"
	"github.com/ruko1202/maintmode/internal/services/calendar"
	"github.com/ruko1202/maintmode/internal/services/usersummary"
)

type Implementation struct {
	calendarSrv    *calendar.Service
	authorizer     *authz.CasbinAuthorizer
	userSummarySrv *usersummary.Service
}

func New(calendarSrv *calendar.Service, authorizer *authz.CasbinAuthorizer, userSummarySrv *usersummary.Service) *Implementation {
	return &Implementation{
		calendarSrv:    calendarSrv,
		authorizer:     authorizer,
		userSummarySrv: userSummarySrv,
	}
}

package uiapprovals

import (
	"github.com/ruko1202/maintmode/internal/services/calendar"
	"github.com/ruko1202/maintmode/internal/services/usersummary"
)

// Implementation serves the "awaiting my approval" screen. No authorizer here:
// the route is gated by the maintenance.approve scenario middleware, and this
// read exposes no per-row actions to resolve.
type Implementation struct {
	calendarSrv    *calendar.Service
	userSummarySrv *usersummary.Service
}

func New(calendarSrv *calendar.Service, userSummarySrv *usersummary.Service) *Implementation {
	return &Implementation{
		calendarSrv:    calendarSrv,
		userSummarySrv: userSummarySrv,
	}
}

package apimaint

import (
	"github.com/ruko1202/maintmode/internal/services/maint"
	"github.com/ruko1202/maintmode/internal/services/usersummary"
)

type Implementation struct {
	maintSrv       *maint.Service
	userSummarySrv *usersummary.Service
}

func New(maintSrv *maint.Service, userSummarySrv *usersummary.Service) *Implementation {
	return &Implementation{
		maintSrv:       maintSrv,
		userSummarySrv: userSummarySrv,
	}
}

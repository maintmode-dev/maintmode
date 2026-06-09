package resourcesapi

import (
	"github.com/ruko1202/maintmode/internal/services/resources"
	"github.com/ruko1202/maintmode/internal/services/usersummary"
)

type Implementation struct {
	resourcesSrv   *resources.Service
	userSummarySrv *usersummary.Service
}

func New(resourcesSrv *resources.Service, userSummarySrv *usersummary.Service) *Implementation {
	return &Implementation{
		resourcesSrv:   resourcesSrv,
		userSummarySrv: userSummarySrv,
	}
}

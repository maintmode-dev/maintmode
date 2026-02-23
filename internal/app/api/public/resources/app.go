package resourcesapi

import (
	"github.com/ruko1202/maintmode/internal/services/resources"
)

type Implementation struct {
	resourcesSrv *resources.Service
}

func New(resourcesSrv *resources.Service) *Implementation {
	return &Implementation{
		resourcesSrv: resourcesSrv,
	}
}

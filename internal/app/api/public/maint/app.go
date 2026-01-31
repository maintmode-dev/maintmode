package apimaint

import (
	"github.com/ruko1202/maintmode/internal/services/maint"
)

type Implementation struct {
	maintSrv *maint.Service
}

func New(maintSrv *maint.Service) *Implementation {
	return &Implementation{
		maintSrv: maintSrv,
	}
}

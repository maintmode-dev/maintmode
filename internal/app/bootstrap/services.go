package bootstrap

import (
	"github.com/ruko1202/maintmode/internal/services/calendar"
	conflictsSvr "github.com/ruko1202/maintmode/internal/services/conflicts"
	maintSrv "github.com/ruko1202/maintmode/internal/services/maint"
	resourcesSrv "github.com/ruko1202/maintmode/internal/services/resources"
)

// Services contains all service layer dependencies
type Services struct {
	Maint     *maintSrv.Service
	Conflicts *conflictsSvr.Service
	Calendar  *calendar.Service
	Resources *resourcesSrv.Service
}

// NewServices creates and initializes all service layer dependencies
func NewServices(stores *Stores) *Services {
	conflictsService := conflictsSvr.NewService(
		stores.Conflicts,
		stores.ConflictSnapshots,
	)

	return &Services{
		Maint: maintSrv.NewService(
			stores.TxManager,
			stores.Maintenances,
			stores.Resources,

			conflictsService,
		),
		Conflicts: conflictsService,
		Calendar: calendar.NewService(
			stores.Maintenances,
			stores.Resources,
			conflictsService,
		),
		Resources: resourcesSrv.NewService(
			stores.TxManager,
			stores.Resources,
		),
	}
}

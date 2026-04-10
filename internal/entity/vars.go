package entity

var allowedStatusTransitions = map[MaintenanceStatus]map[MaintenanceStatus]struct{}{
	MaintenanceStatusDraft: {
		MaintenanceStatusPlanned:   {}, // approve
		MaintenanceStatusCancelled: {}, // cancel
	},

	MaintenanceStatusPlanned: {
		MaintenanceStatusInProgress: {}, // start
		MaintenanceStatusCancelled:  {}, // cancel
	},

	MaintenanceStatusInProgress: {
		MaintenanceStatusCompleted: {}, // complete
		MaintenanceStatusCancelled: {}, // cancel
	},

	// final states
	MaintenanceStatusCancelled: {},
	MaintenanceStatusCompleted: {},
}

func CanTransition(from, to MaintenanceStatus) bool {
	_, ok := allowedStatusTransitions[from][to]
	return ok
}

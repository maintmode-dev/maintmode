package entity

import (
	"time"

	"github.com/google/uuid"
)

type MaintenanceStepStatus string

const (
	MaintenanceStepStatusPlanned   MaintenanceStepStatus = "planned"
	MaintenanceStepStatusStarted   MaintenanceStepStatus = "started"
	MaintenanceStepStatusCompleted MaintenanceStepStatus = "completed"
	MaintenanceStepStatusCanceled  MaintenanceStepStatus = "canceled"
)

func (s MaintenanceStepStatus) IsTerminal() bool {
	return s == MaintenanceStepStatusCompleted || s == MaintenanceStepStatusCanceled
}

var allowedMaintStepStatusTransitions = map[MaintenanceStepStatus]map[MaintenanceStepStatus]struct{}{
	MaintenanceStepStatusPlanned: {
		MaintenanceStepStatusStarted: {}, // start
	},

	MaintenanceStepStatusStarted: {
		MaintenanceStepStatusCompleted: {}, // complete
		MaintenanceStepStatusCanceled:  {}, // cancel
	},

	// final states
	MaintenanceStepStatusCompleted: {}, // complete
	MaintenanceStepStatusCanceled:  {}, // cancel
}

func CanMainStepTransition(from, to MaintenanceStepStatus) bool {
	_, ok := allowedMaintStepStatusTransitions[from][to]
	return ok
}

type MaintenanceStep struct {
	ID                  uuid.UUID
	Order               int32
	Description         string
	RollbackDescription string
	DurationMinutes     int64
	Status              MaintenanceStepStatus
	ActualStartedAt     *time.Time
	ActualCompletedAt   *time.Time
	CreatedAt           time.Time
	UpdatedAt           *time.Time
}

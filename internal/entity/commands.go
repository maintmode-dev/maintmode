package entity

import "github.com/google/uuid"

type CreateMaintenanceCmd struct {
	Title         string
	Description   string
	PlannedPeriod Period
	Scope         MaintenanceScope
	Impact        MaintenanceImpact
	Resources     []*Resource
}

type UpdateMaintenanceCmd struct {
	MaintID       uuid.UUID
	Title         *string
	Description   *string
	PlannedPeriod *Period
	Scope         *MaintenanceScope
	Impact        *MaintenanceImpact
	Resources     []*Resource
}
type StartMaintenanceCmd struct {
	MaintID uuid.UUID
}

type CancelMaintenanceCmd struct {
	MaintID       uuid.UUID
	Reason        MaintenanceCancelReason
	ReasonComment string
}

type CompleteMaintenanceCmd struct {
	MaintID uuid.UUID
}

type ConflictQueryCmd struct {
	MaintID       uuid.UUID
	PlannedPeriod Period
	Scope         MaintenanceScope
	ResourceIDs   []uuid.UUID
}

type SaveConflictsSnapshotCmd struct {
	MaintID          uuid.UUID
	ConflictSnapshot ConflictsSnapshot
}

type ConflictResourcesQueryCmd struct {
	MaintResourceIDs   []uuid.UUID
	ConflictedMaintIDs []uuid.UUID
}

type ApproveMaintenanceCmd struct {
	MaintID               uuid.UUID
	ObservedMaintRevision int64
	ConflictSnapshot      ConflictsSnapshot
}

type CreateResourceCmd struct {
	Name        string
	Description string
	ExternalID  *string
}

package audit

import "github.com/ruko1202/maintmode/internal/entity"

// Maintenance audited actions. Each records a maintenance lifecycle,
// CRUD or step transition the maint service just performed. They mirror the user
// actions in action.go: a concrete type per audit action, implementing the
// Action marker, rendered to a maintenance-entity snapshot by the renderer.
//
// Actor is the user who performed the action, resolved at the API boundary
// (handler). The automatic overdue-cancel path has no human actor and
// publishes MaintCancelled with the synthetic entity.SystemUser as the actor.

// MaintCreated records that Actor created a draft maintenance.
type MaintCreated struct {
	Actor *entity.User
	Maint *entity.Maintenance
}

func (MaintCreated) auditAction() entity.AuditAction { return entity.AuditActionMaintCreated }

// MaintUpdated records that Actor edited a draft maintenance. Changes is the
// before/after diff of the changed scalar fields (+ changed-flag entries for the
// steps/targets collections).
type MaintUpdated struct {
	Actor   *entity.User
	Maint   *entity.Maintenance
	Changes []entity.AuditFieldChange
}

func (MaintUpdated) auditAction() entity.AuditAction { return entity.AuditActionMaintUpdated }

// MaintApproved records that Actor approved a draft (draft → planned).
type MaintApproved struct {
	Actor *entity.User
	Maint *entity.Maintenance
}

func (MaintApproved) auditAction() entity.AuditAction { return entity.AuditActionMaintApproved }

// MaintStarted records that Actor started a maintenance (planned → in_progress).
type MaintStarted struct {
	Actor *entity.User
	Maint *entity.Maintenance
}

func (MaintStarted) auditAction() entity.AuditAction { return entity.AuditActionMaintStarted }

// MaintCompleted records that Actor completed a maintenance (in_progress →
// completed).
type MaintCompleted struct {
	Actor *entity.User
	Maint *entity.Maintenance
}

func (MaintCompleted) auditAction() entity.AuditAction { return entity.AuditActionMaintCompleted }

// MaintCancelled records that Actor canceled a maintenance. The cancel reason
// and comment live on the maintenance snapshot.
type MaintCancelled struct {
	Actor *entity.User
	Maint *entity.Maintenance
}

func (MaintCancelled) auditAction() entity.AuditAction { return entity.AuditActionMaintCanceled }

// MaintStepStarted records that Actor started a maintenance step.
type MaintStepStarted struct {
	Actor *entity.User
	Maint *entity.Maintenance
	Step  *entity.MaintenanceStep
}

func (MaintStepStarted) auditAction() entity.AuditAction {
	return entity.AuditActionMaintStepStarted
}

// MaintStepCompleted records that Actor completed a maintenance step.
type MaintStepCompleted struct {
	Actor *entity.User
	Maint *entity.Maintenance
	Step  *entity.MaintenanceStep
}

func (MaintStepCompleted) auditAction() entity.AuditAction {
	return entity.AuditActionMaintStepCompleted
}

// MaintStepCancelled records that Actor canceled a maintenance step.
type MaintStepCancelled struct {
	Actor *entity.User
	Maint *entity.Maintenance
	Step  *entity.MaintenanceStep
}

func (MaintStepCancelled) auditAction() entity.AuditAction {
	return entity.AuditActionMaintStepCanceled
}

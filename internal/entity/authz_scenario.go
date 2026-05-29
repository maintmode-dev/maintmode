package entity

type AuthzScenario string

const AuthzActExecute = "execute"

// maint scenarios
const (
	AuthzScenarioMaintenanceRead         AuthzScenario = "maintenance.read"
	AuthzScenarioMaintenanceCreate       AuthzScenario = "maintenance.create"
	AuthzScenarioMaintenanceEdit         AuthzScenario = "maintenance.edit"
	AuthzScenarioMaintenanceStart        AuthzScenario = "maintenance.start"
	AuthzScenarioMaintenanceComplete     AuthzScenario = "maintenance.complete"
	AuthzScenarioMaintenanceCancel       AuthzScenario = "maintenance.cancel"
	AuthzScenarioMaintenanceApprove      AuthzScenario = "maintenance.approve"
	AuthzScenarioMaintenanceStepStart    AuthzScenario = "maintenance.step.start"
	AuthzScenarioMaintenanceStepComplete AuthzScenario = "maintenance.step.complete"
	AuthzScenarioMaintenanceStepCancel   AuthzScenario = "maintenance.step.cancel"
)

// calendar scenarios
const (
	AuthzScenarioCalendarRead AuthzScenario = "calendar.read"
)

// resource scenarios
const (
	AuthzScenarioResourceRead    AuthzScenario = "resource.read"
	AuthzScenarioResourceCreate  AuthzScenario = "resource.create"
	AuthzScenarioResourceArchive AuthzScenario = "resource.archive"
)

// auth scenarios
const (
	AuthzScenarioAuthRolesRead     AuthzScenario = "auth.roles.read"
	AuthzScenarioAuthRolesManage   AuthzScenario = "auth.roles.manage"
	AuthzScenarioAuthUserRolesRead AuthzScenario = "auth.user_roles.read"
	AuthzScenarioAuthAuditRead     AuthzScenario = "auth.audit.read"
)

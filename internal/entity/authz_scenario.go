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
	AuthzScenarioResourceRead      AuthzScenario = "resource.read"
	AuthzScenarioResourceCreate    AuthzScenario = "resource.create"
	AuthzScenarioResourceEdit      AuthzScenario = "resource.edit"
	AuthzScenarioResourceArchive   AuthzScenario = "resource.archive"
	AuthzScenarioResourceUnarchive AuthzScenario = "resource.unarchive"
)

// notification scenarios
const (
	AuthzScenarioNotificationChannelCreate    AuthzScenario = "notification.channel.create"
	AuthzScenarioNotificationChannelArchive   AuthzScenario = "notification.channel.archive"
	AuthzScenarioNotificationChannelUnarchive AuthzScenario = "notification.channel.unarchive"
)

// auth scenarios
const (
	AuthzScenarioAuthRolesRead     AuthzScenario = "auth.roles.read"
	AuthzScenarioAuthRolesManage   AuthzScenario = "auth.roles.manage"
	AuthzScenarioAuthUserRolesRead AuthzScenario = "auth.user_roles.read"
	AuthzScenarioAuthAuditRead     AuthzScenario = "auth.audit.read"

	// user management (admin only)
	AuthzScenarioAuthUsersRead   AuthzScenario = "auth.users.read"
	AuthzScenarioAuthUsersManage AuthzScenario = "auth.users.manage"
)

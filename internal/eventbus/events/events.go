// Package events defines the domain events published to the global eventbus.
// Each event implements eventbus.Event; the bus itself stays domain-agnostic.
package events

type EventType string

const (
	EventTypeUserLoginSuccess  EventType = "user.login.succeeded"
	EventTypeUserLoginFailed   EventType = "user.login.failed"
	EventTypeUserLogoutSuccess EventType = "user.logged_out"
	EventTypeUserRolesChanged  EventType = "user.roles.changed"
	EventTypeUserBlocked       EventType = "user.blocked"
	EventTypeUserUnblocked     EventType = "user.unblocked"
)

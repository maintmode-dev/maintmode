package audit

import (
	"fmt"

	"github.com/ruko1202/maintmode/internal/entity"
)

// AuthMethodToggled records that Actor enabled or disabled a built-in sign-in
// method.
//
// The snapshot carries the method, the new state and the actor -- the new state
// rather than a before/after pair, matching IntegrationUpdated: for a boolean,
// "enabled=false" and "changed from true to false" say the same thing, and the
// neighboring actions are already written the first way.
type AuthMethodToggled struct {
	Actor   *entity.User
	Method  entity.AuthMethodName
	Enabled bool
}

func (AuthMethodToggled) auditAction() entity.AuditAction {
	return entity.AuditActionAuthMethodToggled
}

// fillAuthMethodToggledPayload renders the toggle to its snapshot.
//
// EntityType is assigned EXPLICITLY, and that is the whole reason this lives in
// its own function rather than inline. Renderer.Render defaults the field to
// AuditEntityTypeUser and every other arm of fillAuthPayload relies on that
// default, so an arm that simply forgot would produce a perfectly plausible row
// filed against the admin who threw the switch instead of the method they
// changed -- a failure with no symptom.
func fillAuthMethodToggledPayload(payload *entity.ProcessorTaskPayloadAuditWrite, a AuthMethodToggled) {
	setActor(payload, a.Actor)

	payload.EntityType = entity.AuditEntityTypeAuthSetting
	payload.EntityID = string(a.Method)
	payload.Details = fmt.Sprintf("sign-in method %s %s by %s",
		a.Method, enabledVerb(a.Enabled), a.Actor.Email)
}

// enabledVerb renders the new state as something an operator reads while
// scanning, rather than "enabled=false".
//
// The two words stay literals and goconst is silenced rather than obeyed: the
// occurrences it counts are in the render test, which asserts them
// independently on purpose. A test comparing a constant against the same
// constant would pass through any rewording and prove nothing.
//
//nolint:goconst
func enabledVerb(enabled bool) string {
	if enabled {
		return "enabled"
	}

	return "disabled"
}

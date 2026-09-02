// Package audit defines the audited actions a service reports to the durable
// audit trail, and renders them to the point-in-time snapshot the
// audit-write outbox task carries.
//
// These are audit actions, not a general event system: there is one consumer
// (the audit log), so a service calls auditpublisher.Publish(ctx, audit.X{...})
// with the action it just performed. The publisher renders the snapshot and
// enqueues it; the audit-write processor writes audit_log after commit, outside
// any tx.
package audit

import "github.com/ruko1202/maintmode/internal/entity"

// Action marks a value as an audited action. auditAction returns the audit-log
// action it records — also used as the discriminator in logs/spans. Concrete
// actions are the types below; the marker keeps the publisher decoupled from
// each one.
type Action interface {
	auditAction() entity.AuditAction
}

// LoginSuccess records a successful sign-in. Meta carries IP / user agent /
// session id.
type LoginSuccess struct {
	User *entity.User
	Meta *entity.AuditMetadata
}

func (LoginSuccess) auditAction() entity.AuditAction { return entity.AuditActionLoginSuccess }

// LoginFailed records a failed sign-in. Meta carries IP / user agent and a
// whitelist-safe failure reason (never the raw error).
type LoginFailed struct {
	User *entity.User
	Meta *entity.AuditMetadata
}

func (LoginFailed) auditAction() entity.AuditAction { return entity.AuditActionLoginFailed }

// LogoutSuccess records a manual logout.
type LogoutSuccess struct {
	User      *entity.User
	SessionID string
}

func (LogoutSuccess) auditAction() entity.AuditAction { return entity.AuditActionLogoutSuccess }

// RolesChangeKind distinguishes the sub-kind of a roles change: assigned /
// revoked / replaced. It is a classification of the action, not a domain entity.
type RolesChangeKind string

const (
	RolesAssigned RolesChangeKind = "assigned"
	RolesRevoked  RolesChangeKind = "revoked"
	RolesReplaced RolesChangeKind = "replaced"
)

// RolesChange describes which roles a roles-change action touched.
type RolesChange struct {
	Roles   []entity.Role // roles touched by the action (assigned/revoked) or the final set (replaced)
	Added   []entity.Role // replaced only
	Removed []entity.Role // replaced only
}

// RolesChanged records a change to a user's roles. Kind distinguishes
// assigned / revoked / replaced; Change carries the affected roles.
type RolesChanged struct {
	Actor  *entity.User
	Target *entity.User
	Kind   RolesChangeKind
	Change RolesChange
}

func (RolesChanged) auditAction() entity.AuditAction { return entity.AuditActionRolesChanged }

// UserBlocked records that Actor blocked Target.
type UserBlocked struct {
	Actor  *entity.User
	Target *entity.User
}

func (UserBlocked) auditAction() entity.AuditAction { return entity.AuditActionUserBlocked }

// UserUnblocked records that Actor unblocked Target.
type UserUnblocked struct {
	Actor  *entity.User
	Target *entity.User
}

func (UserUnblocked) auditAction() entity.AuditAction { return entity.AuditActionUserUnblocked }

// UserTagsChanged records an admin editing Target's messenger tags via
// PATCH /users/{id}. Changes carries one before/after entry per tag the edit
// actually touched.
//
// Only the admin path is audited. A user editing their own tags through
// PATCH /me leaves no trail on purpose: you change your own quietly, someone
// else's with a trace — the same asymmetry as user.blocked and roles.changed,
// which are all actions taken on another person.
type UserTagsChanged struct {
	Actor   *entity.User
	Target  *entity.User
	Changes []entity.AuditFieldChange
}

func (UserTagsChanged) auditAction() entity.AuditAction { return entity.AuditActionUserTagsChanged }

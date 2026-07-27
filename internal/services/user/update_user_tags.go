package user

import (
	"context"

	"github.com/samber/lo"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
)

// Audit field names for the messenger tags. They match the JSON keys of the
// admin request body so the audit trail names the field the admin actually
// edited, not the Go struct member.
const (
	auditFieldTelegramTag = "telegram_tag"
	auditFieldSlackTag    = "slack_tag"
)

// UpdateUserTags applies the present tag fields of cmd to another user's profile
// and returns the refreshed user. This is the admin path (PATCH /users/{id});
// the self-service equivalent is UpdatePreferences, which shares the validation
// but is deliberately not audited — you change your own tags quietly, someone
// else's with a trace.
//
// Semantics match UpdatePreferences exactly: absent fields are left untouched, a
// value that canonicalizes to the empty string (nil, "", whitespace) is
// persisted as NULL, and a rejected handle returns apperr.ErrInvalidMessengerTag
// (→ HTTP 400) before anything is written.
//
// Auditing is best-effort and published only after the transaction commits, so a
// failed enqueue cannot fail an edit that already landed. The diff is taken
// inside the update closure, before the mutation — that is the only point where
// both the old values (read under FOR UPDATE) and the new ones are available.
// Only fields whose value actually changed enter the diff; if nothing changed
// (an empty patch, or a re-send of the current values) no action is published at
// all, because an audit entry with an empty changes list records nothing.
func (s *Service) UpdateUserTags(ctx context.Context, cmd *entity.UpdateUserTagsCmd) (*entity.User, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.User.UpdateUserTags")
	defer span.End()

	if cmd.IsEmpty() {
		// Nothing to write: skip the transaction entirely and answer with the
		// current profile. A body-less PATCH and a PATCH of "{}" both land here,
		// and neither is an audited event. GetByID still resolves the target, so
		// an unknown id is a 404 rather than a silent 200.
		return s.GetByID(ctx, cmd.UserID)
	}

	var telegramTag, slackTag *string

	if cmd.SetTelegramTag {
		tag, ok := entity.CanonicalTelegramTag(cmd.TelegramTag)
		if !ok {
			xlog.Warn(ctx, "invalid telegram tag")
			return nil, apperr.ErrInvalidMessengerTag
		}
		telegramTag = emptyToNil(tag)
	}

	if cmd.SetSlackTag {
		tag, ok := entity.CanonicalSlackTag(cmd.SlackTag)
		if !ok {
			xlog.Warn(ctx, "invalid slack tag")
			return nil, apperr.ErrInvalidMessengerTag
		}
		slackTag = emptyToNil(tag)
	}

	var changes []entity.AuditFieldChange

	user, err := s.updateWithApply(ctx, cmd.UserID, func(_ context.Context, user *entity.User) error {
		// Snapshot before mutating: after the assignments below the old values
		// are gone, and the audit trail is exactly the before/after pair.
		changes = tagChanges(cmd, user, telegramTag, slackTag)

		if cmd.SetTelegramTag {
			user.TelegramTag = telegramTag
		}
		if cmd.SetSlackTag {
			user.SlackTag = slackTag
		}

		return nil
	})
	if err != nil {
		xlog.Error(ctx, "failed to update user tags", xfield.Error(err))
		return nil, err
	}

	if len(changes) > 0 {
		s.publishAudit(ctx, audit.UserTagsChanged{
			Actor:   cmd.Actor,
			Target:  user,
			Changes: changes,
		})
	}

	return user, nil
}

// tagChanges builds the before/after diff of the tags cmd actually moves, given
// the user as it stands before the mutation and the already-validated new
// values. A field the admin sent unchanged produces no entry: the request
// touched it, the profile did not change, and the audit trail records changes.
func tagChanges(cmd *entity.UpdateUserTagsCmd, before *entity.User, telegramTag, slackTag *string) []entity.AuditFieldChange {
	var changes []entity.AuditFieldChange

	if cmd.SetTelegramTag {
		if change, ok := tagChange(auditFieldTelegramTag, before.TelegramTag, telegramTag); ok {
			changes = append(changes, change)
		}
	}

	if cmd.SetSlackTag {
		if change, ok := tagChange(auditFieldSlackTag, before.SlackTag, slackTag); ok {
			changes = append(changes, change)
		}
	}

	return changes
}

// tagChange renders one before/after entry, reporting false when the value did
// not move. A NULL tag renders as the empty string on either side, which is the
// AuditFieldChange convention for "unset on that side".
func tagChange(field string, oldValue, newValue *string) (entity.AuditFieldChange, bool) {
	before, after := lo.FromPtr(oldValue), lo.FromPtr(newValue)
	if before == after {
		return entity.AuditFieldChange{}, false
	}

	return entity.AuditFieldChange{Field: field, Old: before, New: after}, true
}

package user

import (
	"context"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// UpdatePreferences applies the present fields of cmd to the user's own profile
// and returns the refreshed user. Absent fields are left untouched, so patching
// one preference can never clobber another.
//
// Every present field is validated before anything is written; a rejection
// returns apperr.ErrInvalidTimezone / apperr.ErrInvalidMessengerTag (→ HTTP 400)
// and leaves the row unchanged. An accepted value that canonicalizes to the
// empty string (nil, "", whitespace) is persisted as NULL — the reset.
//
// All fields are applied inside a single updateWithApply, i.e. one transaction
// and one FOR UPDATE on the row. Splitting this per field would take the same
// row lock several times and could leave a partially applied patch behind if the
// request failed between them.
//
// This is a self-service preference change; it is intentionally not audited.
func (s *Service) UpdatePreferences(ctx context.Context, userID uuid.UUID, cmd *entity.UpdatePreferencesCmd) (*entity.User, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.User.UpdatePreferences")
	defer span.End()

	if cmd.IsEmpty() {
		// Nothing to write: skip the transaction entirely and answer with the
		// current profile. A body-less PATCH and a PATCH of "{}" both land here.
		return s.GetByID(ctx, userID)
	}

	var timezone, telegramTag, slackTag *string

	if cmd.SetTimezone {
		name, ok := entity.CanonicalTimezone(cmd.Timezone)
		if !ok {
			xlog.Warn(ctx, "invalid timezone preference")
			return nil, apperr.ErrInvalidTimezone
		}
		timezone = emptyToNil(name)
	}

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

	return s.updateWithApply(ctx, userID, func(_ context.Context, user *entity.User) error {
		if cmd.SetTimezone {
			user.Timezone = timezone
		}
		if cmd.SetTelegramTag {
			user.TelegramTag = telegramTag
		}
		if cmd.SetSlackTag {
			user.SlackTag = slackTag
		}

		return nil
	})
}

// emptyToNil maps an accepted-but-empty canonical value onto the NULL that
// represents "not set" in storage.
func emptyToNil(value string) *string {
	if value == "" {
		return nil
	}

	return &value
}

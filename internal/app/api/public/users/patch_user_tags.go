package users

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/users/models"
	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
)

// UpdateUserTags godoc
// @Summary Update another user's messenger tags
// @Description Updates the messenger handles of the user identified by {id}. Admin-only: this is the counterpart to PATCH /me, for the case where an admin spots a wrong or someone else's handle in the user list and the owner cannot or will not fix it. The edit is recorded in the audit log with the old and new value of every field that actually changed.
// @Description This is a true patch: only the keys physically present in the JSON object are applied. An omitted key leaves that handle untouched, and an explicit null (or an empty/whitespace string) clears it. Sending an empty object, or no body at all, therefore changes nothing and still returns 200 with the current profile.
// @Description A handle is stored exactly as entered — a leading "@" is neither added nor stripped, because the stored value is pasted verbatim into the outgoing notification. An unacceptable handle returns 400 and leaves the whole patch unapplied.
// @Description Note on slack_tag: for Slack the handle is addressing, not a guaranteed push. It makes the notification text name the right person, but a real Slack mention requires the user's Slack user ID, which this field does not carry.
// @Description Timezone is not part of this contract and is rejected as unknown input: it only affects the owner's own display, which is not the admin's to change.
// @Tags Users
// @Accept json
// @Produce json
// @Param id path string true "User ID (UUID)" Format(uuid)
// @Param request body apimodels.UpdateUserTagsRequest true "Messenger tags patch"
// @Success 200 {object} apimodels.User "Updated user"
// @Failure 400 {object} httperrors.ErrorResponse "Invalid user ID or messenger tag"
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 403 {object} httperrors.ErrorResponse "Forbidden"
// @Failure 404 {object} httperrors.ErrorResponse "User not found"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/users/{id} [patch]
func (i *Implementation) UpdateUserTags(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Users.UpdateUserTags")
	defer span.End()
	op := "update user tags"

	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		xlog.Error(ctx, "parse userID failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ErrInvalidUUID)
	}

	actor, ok := xecho.UserFromEchoCtx(c)
	if !ok {
		err := fmt.Errorf("actor not found")
		xlog.Error(ctx, "actor user not found in echo context", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
	}

	// A true patch: the request tracks which keys were physically present, so
	// null ("clear this") stays distinguishable from an absent key ("leave this
	// alone") and one tag's patch cannot clobber the other. A body-less request
	// binds to a request with no keys present and, like "{}", touches nothing.
	var req apimodels.UpdateUserTagsRequest
	if err := c.Bind(&req); err != nil {
		xlog.Error(ctx, "failed to bind request", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ErrParseBody)
	}

	updated, err := i.userSrv.UpdateUserTags(ctx, &entity.UpdateUserTagsCmd{
		Actor:          actor,
		UserID:         userID,
		SetTelegramTag: req.Has(apimodels.FieldTelegramTag),
		TelegramTag:    req.TelegramTag,
		SetSlackTag:    req.Has(apimodels.FieldSlackTag),
		SlackTag:       req.SlackTag,
	})
	if err != nil {
		xlog.Error(ctx, "update user tags failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	apiUser, err := i.toAPIUser(ctx, updated)
	if err != nil {
		xlog.Error(ctx, "failed to render updated user", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	return c.JSON(http.StatusOK, apiUser)
}

// toAPIUser renders one user in the same shape GET /users/list returns, so the
// caller can drop the response straight into the row it just edited.
//
// The two extra facts that shape needs — the system-wide active-admin count
// (for is_last_admin) and the user's linked OAuth providers — are not carried by
// entity.User, and the listing query is the one place that already assembles
// both. Restricting it to this single id reuses that assembly instead of adding
// a second way to compute the same fields.
func (i *Implementation) toAPIUser(ctx context.Context, user *entity.User) (*apimodels.User, error) {
	result, err := i.userSrv.ListUsers(ctx, &entity.ListUsersCmd{
		IDs:    []uuid.UUID{user.ID},
		Limit:  1,
		Offset: 0,
	})
	if err != nil {
		return nil, err
	}

	if len(result.Users) == 0 {
		// The row existed a moment ago (the update just committed against it),
		// so an empty result means it was deleted in between. Report it as gone
		// rather than rendering a half-populated user.
		return nil, apperr.ErrUserNotFound
	}

	return apimodels.ToAPIUser(result.Users[0], result.ActiveAdminCount, result.ProvidersByUser[user.ID]), nil
}

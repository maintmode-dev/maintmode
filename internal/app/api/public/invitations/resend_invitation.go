//nolint:dupl // revoke and resend share the same id-path + actor scaffolding by design; the service calls differ.
package invitations

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
)

// ResendInvitation godoc
// @Summary Resend an invitation
// @Description Rotates the token, extends the expiry, and re-sends the email for a pending invitation. Works only for pending invitations; expired/accepted/revoked return 409. Requires admin privileges.
// @Tags Users
// @Produce json
// @Param id path string true "Invitation ID (UUID)" Format(uuid)
// @Success 204 "Invitation resent"
// @Failure 400 {object} httperrors.ErrorResponse "Invalid UUID"
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 403 {object} httperrors.ErrorResponse "Forbidden"
// @Failure 404 {object} httperrors.ErrorResponse "Invitation not found"
// @Failure 409 {object} httperrors.ErrorResponse "Invitation is not pending (expired/accepted/revoked)"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/users/invitations/{id}/resend [post]
func (i *Implementation) ResendInvitation(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Invitations.ResendInvitation")
	defer span.End()
	op := "resend invitation"

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		xlog.Error(ctx, "parse invitation id failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ErrInvalidUUID)
	}

	actor, ok := xecho.UserFromEchoCtx(c)
	if !ok {
		err := fmt.Errorf("actor not found")
		xlog.Error(ctx, "actor user not found in echo context", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
	}

	if err := i.invSrv.Resend(ctx, &entity.ResendInvitationCmd{Actor: actor, ID: id}); err != nil {
		xlog.Error(ctx, "resend invitation failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	return c.NoContent(http.StatusNoContent)
}

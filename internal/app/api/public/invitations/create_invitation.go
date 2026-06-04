package invitations

import (
	"context"
	"fmt"
	"net/http"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/go-ozzo/ozzo-validation/v4/is"
	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/invitations/models"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

// Invite godoc
// @Summary Invite a new user
// @Description Creates an invitation for an email with pre-assigned roles and sends the invite link. Requires admin privileges.
// @Tags Users
// @Accept json
// @Produce json
// @Param request body apimodels.CreateInvitationRequest true "Invitation request"
// @Success 201 {object} apimodels.CreateInvitationResponse
// @Failure 400 {object} httperrors.ErrorResponse "Invalid email/roles"
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 403 {object} httperrors.ErrorResponse "Forbidden"
// @Failure 409 {object} httperrors.ErrorResponse "User already exists or active invitation conflict"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/users/invite [post]
func (i *Implementation) Invite(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Invitations.Invite")
	defer span.End()
	op := "invite user"

	req := new(apimodels.CreateInvitationRequest)
	if err := c.Bind(req); err != nil {
		xlog.Error(ctx, "bind request failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ErrParseBody)
	}

	if err := validateCreateInvitationRequest(ctx, req); err != nil {
		xlog.Error(ctx, "invalid request", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
	}

	actor, ok := xecho.UserFromEchoCtx(c)
	if !ok {
		err := fmt.Errorf("actor not found")
		xlog.Error(ctx, "actor user not found in echo context", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
	}

	roles, err := apimodels.FromAPIRoles(req.Roles)
	if err != nil {
		xlog.Error(ctx, "invalid roles", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
	}

	inv, err := i.invSrv.Create(ctx, &entity.CreateInvitationCmd{
		Actor: actor,
		Email: req.Email,
		Roles: roles,
	})
	if err != nil {
		xlog.Error(ctx, "create invitation failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	return c.JSON(http.StatusCreated, apimodels.ToAPICreateInvitationResponse(inv, xtime.UTCNow()))
}

func validateCreateInvitationRequest(ctx context.Context, req *apimodels.CreateInvitationRequest) error {
	return validation.ValidateStructWithContext(ctx, req,
		validation.Field(&req.Email, validation.Required, is.EmailFormat),
		validation.Field(&req.Roles, validation.Required, validation.Length(1, 0)),
	)
}

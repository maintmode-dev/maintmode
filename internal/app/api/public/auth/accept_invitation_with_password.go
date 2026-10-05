package auth

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apiauthmodels "github.com/ruko1202/maintmode/internal/app/api/public/auth/models"
	"github.com/ruko1202/maintmode/internal/entity"
)

// AcceptInvitationWithPassword godoc
// @Summary Accept an invitation by setting a password
// @Description Creates the invited user with the invitation's email, sets the password, spends the invitation and grants its roles, then issues a backend token pair carrying those roles. For instances where people sign in with a password rather than through an identity provider.
// @Tags Users
// @Accept json
// @Produce json
// @Param request body apiauthmodels.AcceptInvitationWithPasswordRequest true "Invitation token and the new password"
// @Success 200 {object} apiauthmodels.TokenPairResponse
// @Failure 400 {object} httperrors.ErrorResponse "code: invalid (no live invitation) | invalid request (password outside the policy, malformed body)"
// @Failure 403 {object} httperrors.ErrorResponse "code: method_disabled | seats_limit_exceeded"
// @Failure 409 {object} httperrors.ErrorResponse "code: conflict (an account with this email already exists)"
// @Failure 429 {object} httperrors.ErrorResponse "Rate limit exceeded"
// @Router /api/v1/users/invitations/accept/password [post]
func (i *Implementation) AcceptInvitationWithPassword(c *echo.Context) error {
	const op = "api.Auth.AcceptInvitationWithPassword"
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), op)
	defer span.End()

	body := new(apiauthmodels.AcceptInvitationWithPasswordRequest)
	if err := c.Bind(body); err != nil {
		return httperrors.ToAPIError(c, op, httperrors.ErrParseBody)
	}

	pair, err := i.authSrv.AcceptInvitationWithPassword(ctx, &entity.AcceptInvitationWithPasswordCmd{
		Token:     body.InvitationToken,
		Password:  body.Password,
		ClientIP:  c.RealIP(),
		UserAgent: c.Request().UserAgent(),
	})
	if err != nil {
		return httperrors.ToAPIError(c, op, err)
	}

	c.Response().Header().Set(echo.HeaderCacheControl, "no-store")
	return c.JSON(http.StatusOK, apiauthmodels.ToAPITokenPairResponse(pair))
}

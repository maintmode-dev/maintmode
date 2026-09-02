package invitations

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apiauthmodels "github.com/ruko1202/maintmode/internal/app/api/public/auth/models"
	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/invitations/models"
	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// AcceptInvitation godoc
// @Summary Accept an invitation (public)
// @Description Accepts an invitation by completing OAuth: verifies the token, checks the OAuth email matches the invited email, creates the user with pre-assigned roles, and returns a token pair (like login). PUBLIC — no auth required. All 4xx failures return only a status code ("invalid" / "email_mismatch") with no message, so no invitation detail leaks.
// @Tags Users
// @Accept json
// @Produce json
// @Param request body apimodels.AcceptInvitationRequest true "Accept request"
// @Success 200 {object} apiauthmodels.TokenPairResponse
// @Failure 400 {object} httperrors.ErrorResponse "code: invalid | email_mismatch"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Router /api/v1/users/invitations/accept [post]
func (i *Implementation) AcceptInvitation(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Invitations.AcceptInvitation")
	defer span.End()
	op := "accept invitation"

	req := new(apimodels.AcceptInvitationRequest)
	if err := c.Bind(req); err != nil {
		xlog.Error(ctx, "bind request failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ErrParseBody)
	}

	// An unknown/unsupported provider is just another invalid invitation — the
	// mapper surfaces it as code "invalid" with no detail.
	provider, ok := entity.ParseAuthMethod(req.OAuthPayload.Provider)
	if !ok {
		return httperrors.ToAPIError(c, op, apperr.ErrInvalidInvitation)
	}

	pair, err := i.invSrv.Accept(ctx, &entity.AcceptInvitationCmd{
		Token:    req.InvitationToken,
		Provider: provider,
		IDToken:  req.OAuthPayload.IDToken,
		ClientIP: c.RealIP(),
	})
	if err != nil {
		xlog.Error(ctx, "accept invitation failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	c.Response().Header().Set(echo.HeaderCacheControl, "no-store")
	return c.JSON(http.StatusOK, apiauthmodels.ToAPITokenPairResponse(pair))
}

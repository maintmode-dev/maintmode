package invitations

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/invitations/models"
)

// PreviewInvitation godoc
// @Summary Preview an invitation (public)
// @Description Validates an invitation token and returns only its status plus an optional OAuth-provider hint. PUBLIC — no auth required. Carries no sensitive invitation data. An unknown/empty token reports status "invalid" with HTTP 200 (never 404), so a link holder cannot probe for valid tokens.
// @Tags Users
// @Produce json
// @Param token query string true "Invitation token from the email link"
// @Success 200 {object} apimodels.InvitationPreviewResponse
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Router /api/v1/users/invitations/preview [get]
func (i *Implementation) PreviewInvitation(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Invitations.PreviewInvitation")
	defer span.End()
	op := "preview invitation"

	status, err := i.invSrv.Preview(ctx, c.QueryParam("token"))
	if err != nil {
		xlog.Error(ctx, "preview invitation failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	// suggested_provider is always null in the MVP; the UI falls back to Google.
	return c.JSON(http.StatusOK, &apimodels.InvitationPreviewResponse{
		Status:            string(status),
		SuggestedProvider: nil,
	})
}

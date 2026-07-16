package auth

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apiauthmodels "github.com/ruko1202/maintmode/internal/app/api/public/auth/models"
	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
)

// UpdateMe godoc
// @Summary Update current authenticated user's preferences
// @Description Updates the caller's timezone preference. Accepts an IANA identifier (e.g. "Asia/Nicosia"); null, empty or whitespace resets to browser auto-detect. An invalid identifier returns 400.
// @Tags Auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body apiauthmodels.UpdateMeRequest true "Preferences patch"
// @Success 200 {object} apiauthmodels.MeResponse
// @Failure 400 {object} httperrors.ErrorResponse "Invalid timezone"
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Router /api/v1/me [patch]
// UpdateMe persists the current user's timezone preference and returns the
// refreshed profile (same shape as GET /me).
func (i *Implementation) UpdateMe(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Auth.UpdateMe")
	defer span.End()
	op := "update current user"

	ctxUser, ok := xecho.UserFromEchoCtx(c)
	if !ok {
		xlog.Error(ctx, "missing user in context")
		return httperrors.ToAPIError(c, op, apperr.ErrInvalidAccessToken)
	}

	// A single-field patch: an absent/null/empty timezone all bind to nil and
	// reset the preference to auto-detect (including an empty request body). This
	// is by design while timezone is the only field; adding a second preference
	// here will require presence tracking so one field's patch can't clobber the
	// other.
	var req apiauthmodels.UpdateMeRequest
	if err := c.Bind(&req); err != nil {
		xlog.Error(ctx, "failed to bind request", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ErrParseBody)
	}

	user, err := i.userSrv.UpdateTimezone(ctx, ctxUser.ID, req.Timezone)
	if err != nil {
		xlog.Error(ctx, "failed to update timezone", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	providers, err := i.userSrv.ListConnectedProviders(ctx, ctxUser.ID)
	if err != nil {
		xlog.Error(ctx, "failed to list connected providers", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	return c.JSON(http.StatusOK, apiauthmodels.ToAPIMeResponse(user, providers))
}

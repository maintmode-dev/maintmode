package auth

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apiauthmodels "github.com/ruko1202/maintmode/internal/app/api/public/auth/models"
	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
)

// UpdateMe godoc
// @Summary Update current authenticated user's preferences
// @Description Updates the caller's own preferences: timezone, telegram_tag, slack_tag. This is a true patch — only the keys actually present in the JSON object are applied, an omitted key leaves its preference untouched, and an explicit null resets it. Sending an empty object (or no body) therefore changes nothing. Timezone accepts an IANA identifier (e.g. "Asia/Nicosia"); null, empty or whitespace resets it to browser auto-detect. The messenger tags are stored exactly as entered — a leading "@" is preserved only if typed — and null, empty or whitespace clears them. An invalid timezone or an unacceptable handle returns 400 and leaves the whole patch unapplied.
// @Tags Auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body apiauthmodels.UpdateMeRequest true "Preferences patch"
// @Success 200 {object} apiauthmodels.MeResponse
// @Failure 400 {object} httperrors.ErrorResponse "Invalid timezone or messenger tag"
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Router /api/v1/me [patch]
// UpdateMe persists the current user's preferences and returns the refreshed
// profile (same shape as GET /me).
func (i *Implementation) UpdateMe(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Auth.UpdateMe")
	defer span.End()
	op := "update current user"

	ctxUser, ok := xecho.UserFromEchoCtx(c)
	if !ok {
		xlog.Error(ctx, "missing user in context")
		return httperrors.ToAPIError(c, op, apperr.ErrInvalidAccessToken)
	}

	// A true patch across several preferences: the request tracks which keys were
	// physically present, so null ("reset this") stays distinguishable from an
	// absent key ("leave this alone") and one field's patch cannot clobber
	// another. A body-less request binds to a request with no keys present and,
	// like "{}", touches nothing.
	var req apiauthmodels.UpdateMeRequest
	if err := c.Bind(&req); err != nil {
		xlog.Error(ctx, "failed to bind request", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ErrParseBody)
	}

	cmd := &entity.UpdatePreferencesCmd{
		SetTimezone:    req.Has(apiauthmodels.FieldTimezone),
		Timezone:       req.Timezone,
		SetTelegramTag: req.Has(apiauthmodels.FieldTelegramTag),
		TelegramTag:    req.TelegramTag,
		SetSlackTag:    req.Has(apiauthmodels.FieldSlackTag),
		SlackTag:       req.SlackTag,
	}

	updated, err := i.userSrv.UpdatePreferences(ctx, ctxUser.ID, cmd)
	if err != nil {
		xlog.Error(ctx, "failed to update preferences", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	providers, err := i.userSrv.ListConnectedProviders(ctx, ctxUser.ID)
	if err != nil {
		xlog.Error(ctx, "failed to list connected providers", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	return c.JSON(http.StatusOK, apiauthmodels.ToAPIMeResponse(updated, providers))
}

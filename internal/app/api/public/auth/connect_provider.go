package auth

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apiauthmodels "github.com/ruko1202/maintmode/internal/app/api/public/auth/models"
	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
)

// ConnectProvider godoc
// @Summary Connect an OAuth provider to the current user
// @Description Links an additional sign-in provider to the authenticated user.
// @Description The frontend completes the OAuth flow and posts the provider's ID token here.
// @Tags Auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param provider path string true "OAuth provider" Enums(google, github)
// @Param request body apiauthmodels.ConnectProviderRequest true "Provider ID token"
// @Success 204 "Provider connected"
// @Failure 400 {object} httperrors.ErrorResponse "Invalid provider or OAuth payload"
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 409 {object} httperrors.ErrorResponse "Provider already connected or linked to another user"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Router /api/v1/me/providers/{provider}/connect [post]
func (i *Implementation) ConnectProvider(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Auth.ConnectProvider")
	defer span.End()
	op := "connect provider"

	ctxUser, ok := xecho.UserFromEchoCtx(c)
	if !ok {
		xlog.Error(ctx, "missing user in context")
		return httperrors.ToAPIError(c, op, apperr.ErrInvalidAccessToken)
	}

	provider, err := apiauthmodels.FromAPIConnectableProvider(c.Param("provider"))
	if err != nil {
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
	}

	body := new(apiauthmodels.ConnectProviderRequest)
	if err := c.Bind(body); err != nil {
		xlog.Error(ctx, "failed to bind connect request", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ErrParseBody)
	}
	if body.IDToken == "" {
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(errors.New("id_token is required")))
	}

	err = i.authSrv.ConnectProvider(ctx, &entity.ConnectProviderCmd{
		UserID:   ctxUser.ID,
		Provider: provider,
		IDToken:  body.IDToken,
	})
	if err != nil {
		xlog.Error(ctx, "connect provider failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	return c.NoContent(http.StatusNoContent)
}

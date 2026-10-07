package auth

import (
	"context"
	"net/http"
	"net/url"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apiauthmodels "github.com/ruko1202/maintmode/internal/app/api/public/auth/models"
	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
)

// ConnectProvider godoc
// @Summary Connect an OAuth provider to the current user
// @Description Starts attaching an additional sign-in provider to the authenticated user. The body must be {"mode":"dance"}: the backend runs the provider's dance, and this answers 200 with a RELATIVE link_url to follow (nothing is linked on this request).
// @Description The link_url must be followed by a top-level browser navigation, not fetch: the dance cookies are SameSite=Lax.
// @Tags Auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param provider path string true "Configured provider instance name, e.g. google"
// @Param request body apiauthmodels.ConnectProviderRequest true "Dance mode"
// @Success 200 {object} apiauthmodels.ConnectProviderDanceResponse "Link ticket minted; follow link_url"
// @Failure 400 {object} httperrors.ErrorResponse "Invalid provider or mode"
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
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

	body := new(apiauthmodels.ConnectProviderRequest)
	if err := c.Bind(body); err != nil {
		xlog.Error(ctx, "failed to bind connect request", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ErrParseBody)
	}

	if err := validateConnectProviderRequest(ctx, body); err != nil {
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
	}

	return i.mintLinkTicket(ctx, c, op, ctxUser.ID, c.Param("provider"))
}

// mintLinkTicket answers a dance-mode connect with the URL to follow.
//
// Nothing is linked here. This hands back a ticket; the link happens at the
// callback, after the person has proved to the provider that the account is
// theirs.
func (i *Implementation) mintLinkTicket(
	ctx context.Context,
	c *echo.Context,
	op string,
	userID uuid.UUID,
	provider string,
) error {
	ticket, err := i.authSrv.MintLinkTicket(ctx, userID, provider)
	if err != nil {
		xlog.Error(ctx, "mint link ticket failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	// Built with net/url rather than concatenated: the ticket is opaque and
	// url-safe by construction, but encoding it is what keeps that true if the
	// secret alphabet ever changes.
	link := "/api/v1/login/oauth/" + url.PathEscape(provider) + "/start?" +
		url.Values{paramLink: {ticket}}.Encode()

	return c.JSON(http.StatusOK, apiauthmodels.ConnectProviderDanceResponse{LinkURL: link})
}

// validateConnectProviderRequest requires {"mode":"dance"}. A typo'd mode is
// refused rather than ignored, and Bind drops an unknown field, so a body with
// the old id_token and no mode is refused too.
func validateConnectProviderRequest(ctx context.Context, body *apiauthmodels.ConnectProviderRequest) error {
	return validation.ValidateStructWithContext(ctx, body,
		validation.Field(&body.Mode,
			validation.Required,
			validation.In(apiauthmodels.ConnectProviderDanceMode),
		),
	)
}

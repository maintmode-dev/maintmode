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
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
)

// ConnectProvider godoc
// @Summary Connect an OAuth provider to the current user
// @Description Attaches an additional sign-in provider to the authenticated user, in one of two ways.
// @Description Posting an id_token is the BFF flow: the frontend ran the provider's dance itself, and this links the identity immediately (204).
// @Description Posting {"mode":"dance"} asks the backend to run the dance instead, and answers 200 with a RELATIVE link_url to follow (nothing is linked on this request).
// @Description Exactly one of the two must be present. The link_url must be followed by a top-level browser navigation, not fetch: the dance cookies are SameSite=Lax.
// @Tags Auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param provider path string true "Configured provider instance name, e.g. google"
// @Param request body apiauthmodels.ConnectProviderRequest true "Provider ID token, or dance mode"
// @Success 204 "Provider connected"
// @Success 200 {object} apiauthmodels.ConnectProviderDanceResponse "Link ticket minted; follow link_url"
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

	body := new(apiauthmodels.ConnectProviderRequest)
	if err := c.Bind(body); err != nil {
		xlog.Error(ctx, "failed to bind connect request", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ErrParseBody)
	}

	if err := validateConnectProviderRequest(ctx, body); err != nil {
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
	}

	if body.Mode != "" {
		return i.mintLinkTicket(ctx, c, op, ctxUser.ID, c.Param("provider"))
	}

	err := i.authSrv.ConnectProvider(ctx, &entity.ConnectProviderCmd{
		UserID:   ctxUser.ID,
		Provider: c.Param("provider"),
		IDToken:  body.IDToken,
	})
	if err != nil {
		xlog.Error(ctx, "connect provider failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	return c.NoContent(http.StatusNoContent)
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

// validateConnectProviderRequest refuses a body that does not name EXACTLY one
// of the two flows.
//
// Stated as two mirrored rules rather than one xor expression, because the
// caller has to be told which field to fix. Required-when-the-other-is-empty
// covers a body naming neither; Empty-when-the-other-is-set covers a body
// naming both -- and refusing both-present matters as much as refusing neither:
// silently preferring one would let a caller think it asked for the dance and
// get an id_token link instead.
//
// Bind drops an unknown field, so a typo'd NAME arrives as an empty struct --
// the same shape as an empty body, and refused the same way. A typo'd mode
// VALUE is refused by the In rule rather than ignored, since falling back to
// the id_token branch would answer a caller who asked for the dance with a 400
// about a field they never sent.
func validateConnectProviderRequest(ctx context.Context, body *apiauthmodels.ConnectProviderRequest) error {
	return validation.ValidateStructWithContext(ctx, body,
		validation.Field(&body.IDToken,
			validation.When(body.Mode == "", validation.Required),
			validation.When(body.Mode != "", validation.Empty),
		),
		validation.Field(&body.Mode,
			validation.When(body.IDToken == "", validation.Required),
			validation.When(body.IDToken != "", validation.Empty),
			validation.In(apiauthmodels.ConnectProviderDanceMode),
		),
	)
}

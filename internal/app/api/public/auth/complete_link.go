package auth

import (
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

// CompleteLink godoc
// @Summary Complete a provider link from the account owner's session
// @Description Redeems the one-time link_code a link-mode OAuth callback put in the redirect, given binding_proof: the nonce whose hash /start received as binding. The identity is attached to the CALLER's account, and only if that account minted the link ticket. Single-use: the code is spent by the first attempt whatever its outcome. Every reason it cannot be redeemed by this caller answers the same 400 link_invalid; 401 means only that the access token itself is not valid.
// @Tags Auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body apiauthmodels.CompleteLinkRequest true "The link code and binding proof"
// @Success 204 "Linked"
// @Failure 400 {object} httperrors.ErrorResponse "link_invalid: the code cannot be redeemed by this caller"
// @Failure 401 {object} httperrors.ErrorResponse "Invalid access token"
// @Failure 409 {object} httperrors.ErrorResponse "The provider account cannot be linked to this profile"
// @Router /api/v1/me/providers/link/complete [post]
func (i *Implementation) CompleteLink(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Auth.CompleteLink")
	defer span.End()
	op := "complete link"

	ctxUser, ok := xecho.UserFromEchoCtx(c)
	if !ok {
		xlog.Error(ctx, "missing user in context")
		return httperrors.ToAPIError(c, op, apperr.ErrInvalidAccessToken)
	}

	// A body that will not bind answers like a bad code: telling a prober its
	// JSON was well-formed is the first bit of a guess.
	body := new(apiauthmodels.CompleteLinkRequest)
	if err := c.Bind(body); err != nil || body.LinkCode == "" {
		return httperrors.ToAPIError(c, op, apperr.ErrLinkCodeInvalid)
	}

	err := i.authSrv.CompleteLink(ctx, &entity.CompleteLinkCmd{
		UserID:       ctxUser.ID,
		LinkCode:     body.LinkCode,
		BindingProof: body.BindingProof,
		Meta:         &entity.AuditMetadata{IP: c.RealIP(), UserAgent: c.Request().UserAgent()},
	})
	if err != nil {
		xlog.Warn(ctx, "link was not completed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	return c.NoContent(http.StatusNoContent)
}

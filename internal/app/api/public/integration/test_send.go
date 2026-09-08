package integrationapi

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/integration/models"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/integrationkinds"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
)

// TestEmail godoc
// @Summary Send a test email with the given SMTP settings
// @Description Sends a test message using the settings in the request body, without saving them or recording the result. Answers 204 when the server accepted the message, 502 with the far end's own error when it did not. Secrets must be supplied in the body: a stored password is never substituted. Admin only.
// @Tags Integrations
// @Accept json
// @Produce json
// @Param request body apimodels.TestIntegrationRequest true "Settings to test"
// @Success 204 "The SMTP server accepted the message"
// @Failure 400 {object} httperrors.ErrorResponse
// @Failure 403 {object} httperrors.ErrorResponse
// @Failure 502 {object} httperrors.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/integrations/email/test [post]
func (i *Implementation) TestEmail(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Integration.TestEmail")
	defer span.End()
	op := "test integration"

	req := new(apimodels.TestIntegrationRequest)
	if err := c.Bind(req); err != nil {
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
	}

	actor, ok := xecho.UserFromEchoCtx(c)
	if !ok {
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(fmt.Errorf("actor not found")))
	}

	// The route is pinned to the email kind rather than parameterised: a live
	// probe is defined for SMTP only, and the other kinds get no equivalent.
	if err := i.integrationSrv.Probe(ctx, &entity.ProbeIntegrationCmd{
		Kind:    integrationkinds.Email.Kind(),
		Config:  req.Config,
		Secrets: req.Secrets,
		To:      req.To,
		Actor:   actor,
	}); err != nil {
		xlog.Warn(ctx, "test integration failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	return c.NoContent(http.StatusNoContent)
}

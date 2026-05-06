package apimaint

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/maint/models"
)

// GetMaint godoc
// @Summary Get maintenance by ID
// @Description Returns a maintenance entity by its unique identifier.
// @Tags Maintenances
// @Accept json
// @Produce json
// @Param id path string true "Maintenance ID (UUID)" Format(uuid)
// @Success 200 {object} apimodels.Maintenance
// @Failure 400 {object} httperrors.ErrorResponse "Invalid UUID"
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 403 {object} httperrors.ErrorResponse "Forbidden"
// @Failure 404 {object} httperrors.ErrorResponse "Maintenance not found"
// @Failure 503 {object} httperrors.ErrorResponse "Auth service unavailable"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/maintenances/{id} [get]
func (i *Implementation) GetMaint(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Maint.GetMaint")
	defer span.End()
	op := "get maintenance"

	maintID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		xlog.Error(ctx, "parse maintID failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ErrInvalidUUID)
	}

	maint, err := i.maintSrv.GetMaint(ctx, maintID)
	if err != nil {
		xlog.Error(ctx, "get maintenance failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	return c.JSON(http.StatusOK, apimodels.ToAPIMaintenance(maint))
}

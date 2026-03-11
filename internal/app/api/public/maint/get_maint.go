package apimaint

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/apierrors"
	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/maint/models"
	"github.com/ruko1202/maintmode/internal/apperr"
)

// GetMaint godoc
// @Summary Get maintenance by ID
// @Description Returns a maintenance entity by its unique identifier.
// @Tags Maintenances
// @Accept json
// @Produce json
// @Param id path string true "Maintenance ID (UUID)" Format(uuid)
// @Success 200 {object} apimodels.Maintenance
// @Failure 400 {object} apierrors.ErrorResponse "Invalid UUID"
// @Failure 404 {object} apierrors.ErrorResponse "Maintenance not found"
// @Failure 500 {object} apierrors.ErrorResponse "Internal error"
// @Router /api/v1/maintenances/{id} [get]
func (i *Implementation) GetMaint(c echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Maint.GetMaint")
	defer span.End()
	op := "get maintenance"

	maintID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		xlog.Error(ctx, "parse uuid failed", xfield.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, apierrors.ErrInvalidUUID)
		return c.JSON(statusCode, errResp)
	}

	maint, err := i.maintSrv.Get(ctx, maintID)
	if err != nil {
		xlog.Error(ctx, "get maintenance failed", xfield.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, fmt.Errorf("%w: '%s'", apperr.ErrMaintNotFound, maintID))
		return c.JSON(statusCode, errResp)
	}

	return c.JSON(http.StatusOK, apimodels.ToAPIMaintenance(maint))
}

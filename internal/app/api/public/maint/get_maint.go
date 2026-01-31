package apimaint

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/ruko1202/xlog"
	"go.uber.org/zap"

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
	ctx := xlog.WithOperation(c.Request().Context(), "api.Maint.GetMaint")

	maintID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		xlog.Error(ctx, "parse uuid failed", zap.Error(err))
		return c.JSON(http.StatusBadRequest, apierrors.NewErrorResponse(
			apierrors.ErrInvalidRequest,
			"id must be a valid UUID",
		))
	}

	maint, err := i.maintSrv.Get(ctx, maintID)
	if err != nil {
		xlog.Error(ctx, "get maintenance failed", zap.Error(err))
		if errors.Is(err, apperr.ErrMaintNotFound) {
			return c.JSON(http.StatusNotFound, apierrors.NewErrorResponse(
				apierrors.ErrNotFound,
				fmt.Sprintf("maintenance '%s' not found", maintID),
			))
		}
		return c.JSON(http.StatusInternalServerError, apierrors.NewErrorResponse(
			apierrors.ErrInvalidRequest,
			"get maintenance failed",
		))
	}

	return c.JSON(http.StatusOK, apimodels.ToAPIMaintenance(maint))
}

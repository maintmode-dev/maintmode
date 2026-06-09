package resourcesapi

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/resources/models"
	"github.com/ruko1202/maintmode/internal/entity"
)

// GetResource godoc
// @Summary Get a resource by ID
// @Description Returns a single resource by its unique identifier.
// @Tags Resources
// @Produce json
// @Param id path string true "Resource ID" Format(uuid)
// @Success 200 {object} apimodels.Resource
// @Failure 400 {object} httperrors.ErrorResponse "Invalid resource id"
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 403 {object} httperrors.ErrorResponse "Forbidden"
// @Failure 404 {object} httperrors.ErrorResponse "Resource not found"
// @Failure 503 {object} httperrors.ErrorResponse "Auth service unavailable"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/resource/{id} [get]
func (i *Implementation) GetResource(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Resources.GetResource")
	defer span.End()
	op := "get resource"

	resourceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		xlog.Error(ctx, "parse resourceID failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ErrInvalidUUID)
	}

	resource, err := i.resourcesSrv.GetResourceByID(ctx, resourceID)
	if err != nil {
		xlog.Error(ctx, "get resource failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	// Resolve author/editor profiles from auth in one batch call (degrades to a
	// labeled summary on failure; never errors the read). ResolveMany dedups and
	// drops nil ids, so author==editor or an unedited resource are safe.
	summaries := i.userSummarySrv.ResolveMany(ctx, apimodels.ResourceUserIDs([]*entity.ResourceDetails{resource}))

	return c.JSON(http.StatusOK, apimodels.ToAPIResource(resource,
		lookupSummary(summaries, resource.CreatedByUserID),
		lookupSummary(summaries, resource.UpdatedByUserID),
	))
}

// lookupSummary resolves a (possibly nil) user id against the resolved summary
// index. A nil id (no author/editor) maps to nil.
func lookupSummary(summaries map[uuid.UUID]*entity.UserSummary, id *uuid.UUID) *entity.UserSummary {
	if id == nil {
		return nil
	}
	return summaries[*id]
}

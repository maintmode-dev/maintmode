package resourcesapi

import (
	"context"
	"net/http"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/resources/models"
	"github.com/ruko1202/maintmode/internal/entity"
)

// CreateResource godoc
// @Summary Create a new resource
// @Description Creates a new resource with the provided details
// @Tags Resources
// @Accept json
// @Produce json
// @Param request body apimodels.CreateResourceRequest true "Resource details"
// @Success 200 {object} apimodels.Resource
// @Failure 400 {object} httperrors.ErrorResponse "Invalid request"
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 403 {object} httperrors.ErrorResponse "Forbidden"
// @Failure 409 {object} httperrors.ErrorResponse "Resource already exists"
// @Failure 503 {object} httperrors.ErrorResponse "Auth service unavailable"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/resource/create [post]
func (i *Implementation) CreateResource(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Resources.CreateResource")
	defer span.End()
	op := "create resource"

	req := new(apimodels.CreateResourceRequest)
	if err := c.Bind(req); err != nil {
		xlog.Error(ctx, "bind request failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
	}

	if err := validateCreateResourceRequest(ctx, req); err != nil {
		xlog.Error(ctx, "invalid request", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
	}

	resource, err := i.resourcesSrv.CreateResource(ctx, &entity.CreateResourceCmd{
		Name:        req.Name,
		Description: req.Description,
		ExternalID:  req.ExternalID,
	})
	if err != nil {
		xlog.Error(ctx, "create resource failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	return c.JSON(http.StatusOK, apimodels.ToAPIResource(resource))
}

func validateCreateResourceRequest(ctx context.Context, req *apimodels.CreateResourceRequest) error {
	return validation.ValidateStructWithContext(ctx, req,
		validation.Field(&req.Name, validation.Required),
		validation.Field(&req.Description, validation.Required),
	)
}

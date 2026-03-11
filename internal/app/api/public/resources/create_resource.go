package resourcesapi

import (
	"context"
	"net/http"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/labstack/echo/v4"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/apierrors"
	apismodels "github.com/ruko1202/maintmode/internal/app/api/public/resources/models"
	"github.com/ruko1202/maintmode/internal/entity"
)

// CreateResource godoc
// @Summary Create a new resource
// @Description Creates a new resource with the provided details
// @Tags Resources
// @Accept json
// @Produce json
// @Param request body apismodels.CreateResourceRequest true "Resource details"
// @Success 200 {object} apismodels.Resource
// @Failure 400 {object} apierrors.ErrorResponse "Invalid request"
// @Failure 409 {object} apierrors.ErrorResponse "Resource already exists"
// @Failure 500 {object} apierrors.ErrorResponse "Internal error"
// @Router /api/v1/resource/create [post]
func (i *Implementation) CreateResource(c echo.Context) error {
	ctx := xlog.WithOperation(c.Request().Context(), "api.Resources.CreateResource")
	op := "create resource"

	req := new(apismodels.CreateResourceRequest)
	if err := c.Bind(req); err != nil {
		xlog.Error(ctx, "bind request failed", xfield.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, apierrors.ValidationErr(err))
		return c.JSON(statusCode, errResp)
	}

	if err := validateCreateResourceRequest(ctx, req); err != nil {
		xlog.Error(ctx, "invalid request", xfield.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, apierrors.ValidationErr(err))
		return c.JSON(statusCode, errResp)
	}

	resource, err := i.resourcesSrv.CreateResource(ctx, &entity.CreateResourceCmd{
		Name:        req.Name,
		Description: req.Description,
		ExternalID:  req.ExternalID,
	})
	if err != nil {
		xlog.Error(ctx, "create resource failed", xfield.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, apierrors.ValidationErr(err))
		return c.JSON(statusCode, errResp)
	}

	return c.JSON(http.StatusOK, apismodels.ToAPIResource(resource))
}

func validateCreateResourceRequest(ctx context.Context, req *apismodels.CreateResourceRequest) error {
	return validation.ValidateStructWithContext(ctx, req,
		validation.Field(&req.Name, validation.Required),
		validation.Field(&req.Description, validation.Required),
	)
}

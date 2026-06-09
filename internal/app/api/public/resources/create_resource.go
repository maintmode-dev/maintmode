package resourcesapi

import (
	"context"
	"fmt"
	"net/http"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/resources/models"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
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

	author, ok := xecho.UserFromEchoCtx(c)
	if !ok {
		err := fmt.Errorf("author not found")
		xlog.Error(ctx, "author user not found in echo context", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
	}

	resource, err := i.resourcesSrv.CreateResource(ctx, &entity.CreateResourceCmd{
		Name:            req.Name,
		Description:     req.Description,
		ExternalID:      req.ExternalID,
		CreatedByUserID: author.ID,
	})
	if err != nil {
		xlog.Error(ctx, "create resource failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	// CreateResource is get-or-create: on a name clash it returns the existing
	// resource, whose author/editor may be someone else. Resolve authorship from
	// the returned row rather than assuming the caller, so an idempotent create
	// renders the real author. ResolveMany degrades to a labeled summary on auth
	// failure and never errors the response.
	summaries := i.userSummarySrv.ResolveMany(ctx, apimodels.ResourceUserIDs([]*entity.ResourceDetails{resource}))

	return c.JSON(http.StatusOK, apimodels.ToAPIResource(resource,
		lookupSummary(summaries, resource.CreatedByUserID),
		lookupSummary(summaries, resource.UpdatedByUserID),
	))
}

func validateCreateResourceRequest(ctx context.Context, req *apimodels.CreateResourceRequest) error {
	return validation.ValidateStructWithContext(ctx, req,
		validation.Field(&req.Name, validation.Required),
		validation.Field(&req.Description, validation.Required),
	)
}

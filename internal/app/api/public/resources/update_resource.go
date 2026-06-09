package resourcesapi

import (
	"context"
	"fmt"
	"net/http"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/resources/models"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
)

// UpdateResource godoc
// @Summary Update a resource
// @Description Partially updates a resource. All body fields are optional; an
// @Description omitted field is left unchanged. external_id may be set to an
// @Description empty string to clear it.
// @Tags Resources
// @Accept json
// @Produce json
// @Param id path string true "Resource ID" Format(uuid)
// @Param request body apimodels.UpdateResourceRequest true "Fields to update"
// @Success 200 {object} apimodels.Resource
// @Failure 400 {object} httperrors.ErrorResponse "Invalid request"
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 403 {object} httperrors.ErrorResponse "Forbidden"
// @Failure 404 {object} httperrors.ErrorResponse "Resource not found"
// @Failure 409 {object} httperrors.ErrorResponse "Resource name already exists"
// @Failure 503 {object} httperrors.ErrorResponse "Auth service unavailable"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/resource/{id} [patch]
func (i *Implementation) UpdateResource(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Resources.UpdateResource")
	defer span.End()
	op := "update resource"

	resourceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		xlog.Error(ctx, "parse resourceID failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ErrInvalidUUID)
	}

	req := new(apimodels.UpdateResourceRequest)
	if err := c.Bind(req); err != nil {
		xlog.Error(ctx, "bind request failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
	}

	if err := validateUpdateResourceRequest(ctx, req); err != nil {
		xlog.Error(ctx, "invalid request", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
	}

	editor, ok := xecho.UserFromEchoCtx(c)
	if !ok {
		err := fmt.Errorf("editor not found")
		xlog.Error(ctx, "editor user not found in echo context", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
	}

	resource, err := i.resourcesSrv.UpdateResource(ctx, &entity.UpdateResourceCmd{
		ID:              resourceID,
		Name:            req.Name,
		Description:     req.Description,
		ExternalID:      req.ExternalID,
		UpdatedByUserID: editor.ID,
	})
	if err != nil {
		xlog.Error(ctx, "update resource failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	// Resolve only the author from auth — the editor is the authenticated caller,
	// so render its summary directly. ResolveOne degrades to a labeled summary on
	// failure and never errors the write result.
	author := i.userSummarySrv.ResolveOne(ctx, lo.FromPtr(resource.CreatedByUserID))

	return c.JSON(http.StatusOK, apimodels.ToAPIResource(resource, author, editor.ToUserSummary()))
}

func validateUpdateResourceRequest(ctx context.Context, req *apimodels.UpdateResourceRequest) error {
	// NilOrNotEmpty allows an omitted (nil) field but rejects an empty string,
	// so name/description can't be cleared. external_id has no rule: it may be
	// set to an empty string to clear it.
	return validation.ValidateStructWithContext(ctx, req,
		validation.Field(&req.Name, validation.NilOrNotEmpty),
		validation.Field(&req.Description, validation.NilOrNotEmpty),
	)
}

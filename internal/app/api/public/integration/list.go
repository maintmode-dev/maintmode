package integrationapi

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/integration/models"
	"github.com/ruko1202/maintmode/internal/entity"
)

// List godoc
// @Summary List integrations
// @Description Returns all integrations (masked). Admin only.
// @Tags Integrations
// @Produce json
// @Success 200 {object} apimodels.ListIntegrationsResponse
// @Failure 403 {object} httperrors.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/integrations [get]
func (i *Implementation) List(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Integration.List")
	defer span.End()
	op := "list integrations"

	items, err := i.integrationSrv.List(ctx)
	if err != nil {
		return httperrors.ToAPIError(c, op, err)
	}

	summaries := i.userSummarySrv.ResolveMany(ctx, userIDs(items))
	return c.JSON(http.StatusOK, &apimodels.ListIntegrationsResponse{
		Integrations: apimodels.ToAPIIntegrations(items, summaries),
	})
}

func userIDs(items []*entity.MaskedIntegration) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(items)*2)
	for _, m := range items {
		ids = append(ids,
			lo.FromPtr(m.CreatedByUserID),
			lo.FromPtr(m.UpdatedByUserID),
		)
	}
	return lo.Filter(ids, func(id uuid.UUID, _ int) bool { return id != uuid.Nil })
}

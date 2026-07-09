// Package integrationapi is the admin-only HTTP layer for the integration
// registry (RUK-196). It binds requests, threads the authenticated admin actor,
// calls the integration service, maps domain errors to HTTP, and resolves
// authorship on read. It never surfaces a plaintext or ciphertext secret — the
// service returns a masked view and this layer passes it through.
package integrationapi

import (
	"context"

	"github.com/samber/lo"

	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/integration/models"
	"github.com/ruko1202/maintmode/internal/entity"
	integrationsvc "github.com/ruko1202/maintmode/internal/services/integration"
	"github.com/ruko1202/maintmode/internal/services/usersummary"
)

type Implementation struct {
	integrationSrv *integrationsvc.Service
	userSummarySrv *usersummary.Service
}

func New(integrationSrv *integrationsvc.Service, userSummarySrv *usersummary.Service) *Implementation {
	return &Implementation{
		integrationSrv: integrationSrv,
		userSummarySrv: userSummarySrv,
	}
}

// toAPIWithAuthorship maps a single masked integration to its API shape,
// resolving the author and editor summaries from the auth service.
func (i *Implementation) toAPIWithAuthorship(ctx context.Context, m *entity.MaskedIntegration) *apimodels.Integration {
	author := i.userSummarySrv.ResolveOne(ctx, lo.FromPtr(m.CreatedByUserID))
	editor := i.userSummarySrv.ResolveOne(ctx, lo.FromPtr(m.UpdatedByUserID))

	return apimodels.ToAPIIntegration(m, author, editor)
}

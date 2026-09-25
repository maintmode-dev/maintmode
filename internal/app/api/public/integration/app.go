// Package integrationapi is the admin-only HTTP layer for the integration
// registry. It binds requests, threads the authenticated admin actor,
// calls the integration service, maps domain errors to HTTP, and resolves
// authorship on read. It never surfaces a plaintext or ciphertext secret — the
// service returns a masked view and this layer passes it through.
package integrationapi

import (
	"context"

	"github.com/samber/lo"

	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/integration/models"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/integrationkinds"
	integrationsvc "github.com/ruko1202/maintmode/internal/services/integration"
	"github.com/ruko1202/maintmode/internal/services/usersummary"
)

// LoginHealth reports whether a configured login provider can actually be used.
//
// Declared consumer-side over the live snapshot: this handler asks about one
// provider and must not reach into the auth module for anything more.
type LoginHealth interface {
	ProviderHealth(name string) string
}

type Implementation struct {
	integrationSrv *integrationsvc.Service
	userSummarySrv *usersummary.Service
	loginHealth    LoginHealth
}

// New builds the handler. loginHealth is the live provider health the read
// paths surface for login kinds.
func New(
	integrationSrv *integrationsvc.Service,
	userSummarySrv *usersummary.Service,
	loginHealth LoginHealth,
) *Implementation {
	return &Implementation{
		integrationSrv: integrationSrv,
		userSummarySrv: userSummarySrv,
		loginHealth:    loginHealth,
	}
}

// toAPIWithAuthorship maps a single masked integration to its API shape,
// resolving the author and editor summaries from the auth service.
func (i *Implementation) toAPIWithAuthorship(ctx context.Context, m *entity.MaskedIntegration) *apimodels.Integration {
	author := i.userSummarySrv.ResolveOne(ctx, lo.FromPtr(m.CreatedByUserID))
	editor := i.userSummarySrv.ResolveOne(ctx, lo.FromPtr(m.UpdatedByUserID))

	return apimodels.ToAPIIntegration(m, author, editor, i.healthOf(m))
}

// healthOf reports the live health of a login provider, or "" for anything
// else -- a delivery integration has no IdP behind it to be unreachable, and
// its reachability is already reported per channel elsewhere.
//
// Read from the snapshot rather than probed here: probing on an admin GET would
// hand the operator a request that waits out a discovery timeout.
func (i *Implementation) healthOf(m *entity.MaskedIntegration) string {
	if m.Kind != integrationkinds.CategoryLogin {
		return ""
	}

	return i.loginHealth.ProviderHealth(m.Name)
}

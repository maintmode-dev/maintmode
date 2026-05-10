package authz

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

func (a *CasbinAuthorizer) Allow(ctx context.Context, roles []entity.Role, scenario entity.AuthzScenario) (bool, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Authz.Allow",
		xfield.String("scenario", string(scenario)),
	)
	defer span.End()

	for _, role := range roles {
		if !role.Valid(ctx) {
			continue
		}

		allowed, err := a.enforcer.Enforce(string(role), string(scenario), entity.AuthzActExecute)
		if err != nil {
			xlog.Error(ctx, "failed to enforce policy", xfield.Error(err))
			return false, fmt.Errorf("enforce policy: %w", err)
		}
		if allowed {
			return true, nil
		}
	}

	return false, nil
}

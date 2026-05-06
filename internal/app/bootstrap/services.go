package bootstrap

import (
	"context"
	"fmt"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/services/authz"
	"github.com/ruko1202/maintmode/internal/services/calendar"
	conflictsSvr "github.com/ruko1202/maintmode/internal/services/conflicts"
	"github.com/ruko1202/maintmode/internal/services/jwtverifier"
	maintSrv "github.com/ruko1202/maintmode/internal/services/maint"
	resourcesSrv "github.com/ruko1202/maintmode/internal/services/resources"
)

// Services contains all service layer dependencies
type Services struct {
	Maint       *maintSrv.Service
	Conflicts   *conflictsSvr.Service
	Calendar    *calendar.Service
	Resources   *resourcesSrv.Service
	RBAC        *authz.CasbinAuthorizer
	JWTVerifier *jwtverifier.Service
}

// NewServices creates and initializes all service layer dependencies
func NewServices(ctx context.Context, cfg *config.AppConfig, stores *Stores) (*Services, error) {
	conflictsService := conflictsSvr.NewService(
		stores.Conflicts,
		stores.ConflictSnapshots,
	)

	jwtVerifier, err := jwtverifier.NewService(ctx, cfg.JWTVerifier)
	if err != nil {
		return nil, fmt.Errorf("failed to init jwt verifier: %w", err)
	}

	authorizer, err := authz.NewCasbinAuthorizer(cfg.RBAC)
	if err != nil {
		return nil, fmt.Errorf("failed to init casbin authorizer: %w", err)
	}

	return &Services{
		Maint: maintSrv.NewService(
			stores.TxManager,
			stores.Maintenances,
			stores.Resources,

			conflictsService,
		),
		Conflicts: conflictsService,
		Calendar: calendar.NewService(
			stores.Maintenances,
			stores.Resources,
			conflictsService,
		),
		Resources: resourcesSrv.NewService(
			stores.TxManager,
			stores.Resources,
		),
		RBAC:        authorizer,
		JWTVerifier: jwtVerifier,
	}, nil
}

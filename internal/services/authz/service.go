package authz

import (
	"fmt"

	"github.com/casbin/casbin/v3"

	"github.com/ruko1202/maintmode/internal/config"
)

type CasbinAuthorizer struct {
	enforcer *casbin.SyncedEnforcer
}

func NewCasbinAuthorizer(cfg config.RbacConfig) (*CasbinAuthorizer, error) {
	adapter, err := config.NewCasbinAdapter(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to init casbin adapter: %w", err)
	}

	enforcer, err := casbin.NewSyncedEnforcer(cfg.ModelPath, adapter)
	if err != nil {
		return nil, fmt.Errorf("create casbin enforcer: %w", err)
	}

	return &CasbinAuthorizer{enforcer: enforcer}, nil
}

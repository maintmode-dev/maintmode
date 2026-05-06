package bootstrap

import (
	"fmt"

	"github.com/ruko1202/maintmode/internal/config"
	authgateway "github.com/ruko1202/maintmode/internal/gateways/auth"
)

// Gateways contains all external gateways layer dependencies
type Gateways struct {
	Auth *authgateway.Service
}

func NewGateways(cfg *config.AppConfig) (*Gateways, error) {
	autCfg, ok := cfg.ExternalServices["auth"]
	if !ok {
		return nil, fmt.Errorf("auth external service config is missing")
	}
	auth := authgateway.NewService(autCfg)

	return &Gateways{
		Auth: auth,
	}, nil
}

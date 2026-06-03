package bootstrap

import (
	"fmt"

	"github.com/ruko1202/maintmode/internal/config"
	authgateway "github.com/ruko1202/maintmode/internal/gateways/auth"
	"github.com/ruko1202/maintmode/internal/gateways/notifytransport"
	"github.com/ruko1202/maintmode/internal/gateways/notifytransport/slack"
	"github.com/ruko1202/maintmode/internal/gateways/notifytransport/telegram"
)

// Gateways contains all external gateways layer dependencies
type Gateways struct {
	Auth                    *authgateway.Gateway
	NotifyTransportRegistry *notifytransport.Registry
}

func NewGateways(cfg *config.AppConfig) (*Gateways, error) {
	autCfg, ok := cfg.ExternalServices["auth"]
	if !ok {
		return nil, fmt.Errorf("auth external service config is missing")
	}

	registry, err := notifyTransportRegistry(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to init registry: %w", err)
	}

	authGW, err := authgateway.New(autCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to init auth gateway: %w", err)
	}

	return &Gateways{
		Auth:                    authGW,
		NotifyTransportRegistry: registry,
	}, nil
}

func notifyTransportRegistry(cfg *config.AppConfig) (*notifytransport.Registry, error) {
	transports := make([]notifytransport.Transport, 0)
	msgCfg := cfg.NotifyTransport

	if slackCfg := msgCfg.Slack; slackCfg.Enabled {
		cl := slack.New(slackCfg)
		transports = append(transports, cl)
	}

	if telegramCfg := msgCfg.Telegram; telegramCfg.Enabled {
		cl, err := telegram.New(telegramCfg)
		if err != nil {
			return nil, fmt.Errorf("telegram: %w", err)
		}
		transports = append(transports, cl)
	}

	return notifytransport.NewRegistry(cfg, transports...), nil
}

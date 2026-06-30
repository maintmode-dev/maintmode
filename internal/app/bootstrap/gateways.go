package bootstrap

import (
	"fmt"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/gateways/notifytransport"
	emailtransport "github.com/ruko1202/maintmode/internal/gateways/notifytransport/email"
	"github.com/ruko1202/maintmode/internal/gateways/notifytransport/slack"
	"github.com/ruko1202/maintmode/internal/gateways/notifytransport/telegram"
)

// Gateways contains all external gateways layer dependencies. The only external
// surface is the notify-transport registry the messaging/invitation outboxes
// deliver through; auth is an in-process module, not a gateway.
type Gateways struct {
	NotifyTransportRegistry *notifytransport.Registry
}

func NewGateways(cfg *config.AppConfig) (*Gateways, error) {
	registry, err := notifyTransportRegistry(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to init registry: %w", err)
	}

	return &Gateways{
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

	if emailCfg := msgCfg.Email; emailCfg.Enabled {
		cl, err := emailtransport.New(emailCfg)
		if err != nil {
			return nil, fmt.Errorf("email: %w", err)
		}
		transports = append(transports, cl)
	}

	return notifytransport.NewRegistry(cfg, transports...), nil
}

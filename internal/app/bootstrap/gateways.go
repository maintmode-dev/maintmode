package bootstrap

import (
	"fmt"

	"github.com/ruko1202/maintmode/internal/config"
	authgateway "github.com/ruko1202/maintmode/internal/gateways/auth"
	gwmsg "github.com/ruko1202/maintmode/internal/gateways/messengers"
	"github.com/ruko1202/maintmode/internal/gateways/messengers/slack"
	"github.com/ruko1202/maintmode/internal/gateways/messengers/telegram"
)

// Gateways contains all external gateways layer dependencies
type Gateways struct {
	Auth       *authgateway.Gateway
	Messengers *gwmsg.MessengerRegistry
}

func NewGateways(cfg *config.AppConfig) (*Gateways, error) {
	autCfg, ok := cfg.ExternalServices["auth"]
	if !ok {
		return nil, fmt.Errorf("auth external service config is missing")
	}

	messengers, err := newMessengers(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to init messengers: %w", err)
	}

	return &Gateways{
		Auth:       authgateway.New(autCfg),
		Messengers: messengers,
	}, nil
}

func newMessengers(cfg *config.AppConfig) (*gwmsg.MessengerRegistry, error) {
	transports := make([]gwmsg.Messenger, 0)
	msgCfg := cfg.Messengers

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

	return gwmsg.NewMessengerRegistry(cfg, transports...), nil
}

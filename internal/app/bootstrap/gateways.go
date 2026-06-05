package bootstrap

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	authgateway "github.com/ruko1202/maintmode/internal/gateways/auth"
	"github.com/ruko1202/maintmode/internal/gateways/notifytransport"
	"github.com/ruko1202/maintmode/internal/gateways/notifytransport/slack"
	"github.com/ruko1202/maintmode/internal/gateways/notifytransport/telegram"
)

// AuthGateway is the auth S2S surface the service layer depends on. It is an
// interface (satisfied by *authgateway.Gateway) so tests can substitute a fake
// without standing up the real auth service. It composes exactly the methods the
// services consume; the per-service consumer interfaces (e.g.
// maint.ApproverValidator) remain the narrow contracts each service declares.
type AuthGateway interface {
	IsEligibleApprover(ctx context.Context, id uuid.UUID) (bool, error)
	ListActiveUsers(ctx context.Context, q *entity.ListAssignableUsersQuery) (*entity.ListAssignableUsersResult, error)
	GetUsersByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*entity.User, error)
	Introspect(ctx context.Context, tokenString string) (*entity.AccessClaims, error)
}

// Gateways contains all external gateways layer dependencies
type Gateways struct {
	Auth                    AuthGateway
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

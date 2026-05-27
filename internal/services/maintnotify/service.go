package maintnotify

import (
	"fmt"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/services/maintnotify/render"
	"github.com/ruko1202/maintmode/internal/services/maintnotify/router"
	"github.com/ruko1202/maintmode/internal/services/messaging/sender"
)

// Service turns maintenance-lifecycle events into Sender.SendAsync calls
type Service struct {
	frontendURL string
	router      *router.Router
	renderer    *render.Service
	sender      *sender.Service
}

// NewNotifier returns nil, nil when routes is empty — nothing to
// notify about, no notifier needed.
func NewNotifier(
	cfg *config.AppConfig,
	messageSender *sender.Service,
) (*Service, error) {
	rtr, err := router.NewRouter(cfg.MaintNotify.Routes)
	if err != nil {
		return nil, fmt.Errorf("init router: %w", err)
	}

	rend, err := render.New()
	if err != nil {
		return nil, fmt.Errorf("init renderer: %w", err)
	}

	return &Service{
		frontendURL: cfg.App.FrontendURL,
		router:      rtr,
		renderer:    rend,
		sender:      messageSender,
	}, nil
}

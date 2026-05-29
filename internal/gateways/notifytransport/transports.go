package notifytransport

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	stubtransport "github.com/ruko1202/maintmode/internal/gateways/notifytransport/stub"
)

type Transport interface {
	TransportID() entity.NotifyTransport
	Send(ctx context.Context, target string, msg entity.NotifyMessage) error
}

type Registry struct {
	useStub  bool
	registry map[entity.NotifyTransport]Transport
}

func NewRegistry(
	cfg *config.AppConfig,
	tr ...Transport,
) *Registry {
	registry := lo.SliceToMap(tr, func(item Transport) (entity.NotifyTransport, Transport) {
		return item.TransportID(), item
	})
	registry[entity.NotifyTransportStub] = stubtransport.New()

	return &Registry{
		useStub:  cfg.Environment.IsDev() && cfg.NotifyTransport.UseStub,
		registry: registry,
	}
}

func (m *Registry) Get(ctx context.Context, name entity.NotifyTransport) (Transport, error) {
	if m.useStub {
		// Warn on every Get so each delivery in logs carries an
		// explicit "this went to stub, not the real transport"
		// marker — saves time when diagnosing missing notifications.
		xlog.Warn(ctx, "using stub transport", xfield.String("requested", string(name)))
		stub, ok := m.registry[entity.NotifyTransportStub]
		if !ok {
			return nil, fmt.Errorf("stub transport not found")
		}
		return stub, nil
	}

	tr, ok := m.registry[name]
	if !ok {
		return nil, fmt.Errorf("unsupported or disabled transport: %s", name)
	}

	return tr, nil
}

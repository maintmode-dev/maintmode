package messengers

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	stubmessenger "github.com/ruko1202/maintmode/internal/gateways/messengers/stub"
)

type Messenger interface {
	MessengerID() entity.MessengerID
	Send(ctx context.Context, target string, msg entity.Message) error
}

type MessengerRegistry struct {
	useStub  bool
	registry map[entity.MessengerID]Messenger
}

func NewMessengerRegistry(
	cfg *config.AppConfig,
	tr ...Messenger,
) *MessengerRegistry {
	registry := lo.SliceToMap(tr, func(item Messenger) (entity.MessengerID, Messenger) {
		return item.MessengerID(), item
	})
	registry[entity.MessengerStub] = stubmessenger.New()

	return &MessengerRegistry{
		useStub:  cfg.Environment.IsDev() && cfg.Messengers.UseStub,
		registry: registry,
	}
}

func (m *MessengerRegistry) Get(ctx context.Context, name entity.MessengerID) (Messenger, error) {
	if m.useStub {
		// Warn on every Get so each delivery in logs carries an
		// explicit "this went to stub, not the real messenger"
		// marker — saves time when diagnosing missing notifications.
		xlog.Warn(ctx, "using stub messenger", xfield.String("requested", string(name)))
		stub, ok := m.registry[entity.MessengerStub]
		if !ok {
			return nil, fmt.Errorf("stub messenger not found")
		}
		return stub, nil
	}

	tr, ok := m.registry[name]
	if !ok {
		return nil, fmt.Errorf("unsupported or disabled messenger: %s", name)
	}

	return tr, nil
}

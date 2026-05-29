package notifychannel

import (
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xcollection"
)

type Store struct {
	channels *xcollection.MUMap[string, *entity.NotifyChannel]
}

func New(cfg *config.AppConfig) *Store {
	s := &Store{
		channels: xcollection.NewMUMap[string, *entity.NotifyChannel](),
	}

	useStubInDev := cfg.Environment.IsDev() && cfg.NotifyTransport.UseStub

	if cfg.NotifyTransport.Slack.Enabled || useStubInDev {
		s.fillFromConfig(entity.NotifyTransportSlack, cfg.NotifyTransport.Slack.Channels)
	}

	if cfg.NotifyTransport.Telegram.Enabled || useStubInDev {
		s.fillFromConfig(entity.NotifyTransportTelegram, cfg.NotifyTransport.Telegram.Channels)
	}

	return s
}

func (s *Store) fillFromConfig(transport entity.NotifyTransport, transportChan []config.TransportChannel) {
	lo.ForEach(transportChan, func(item config.TransportChannel, _ int) {
		if _, err := s.add(&entity.NotifyChannel{
			Transport:          transport,
			TransportChannelID: item.ID,
			Name:               item.Name,
			Description:        item.Description,
		}); err != nil {
			panic(err)
		}
	})
}

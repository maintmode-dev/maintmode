package transportresolver

import (
	"fmt"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/gateways/notifytransport"
	emailtransport "github.com/ruko1202/maintmode/internal/gateways/notifytransport/email"
	slacktransport "github.com/ruko1202/maintmode/internal/gateways/notifytransport/slack"
	telegramtransport "github.com/ruko1202/maintmode/internal/gateways/notifytransport/telegram"
	"github.com/ruko1202/maintmode/internal/integrationkinds"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

// Builder constructs a notify transport client from a kind's typed Settings —
// the delivery-side half of a kind (its registry half is Parse/Validate). The
// plaintext secrets inside Settings live only for the span of the call; the
// built client closes over whatever it needs.
type Builder func(settings integrationkinds.Settings) (notifytransport.Transport, error)

// Builders returns the production transport -> builder mapping. This — not the
// integration registry — is where "which client serves a transport" is known:
// adding a delivery-capable kind means registering its settings half in the
// kind registry and its builder here, under the matching entity.NotifyTransport
// (the kind's name is derived from that constant by construction; the wiring is
// pinned by TestBuilders_AlignWithIntegrationKinds). A
// kind registered without a builder is a valid non-delivery integration (it
// just never resolves a transport).
func Builders() map[entity.NotifyTransport]Builder {
	return map[entity.NotifyTransport]Builder{
		entity.NotifyTransportSlack:    buildSlack,
		entity.NotifyTransportTelegram: buildTelegram,
		entity.NotifyTransportEmail:    buildEmail,
	}
}

func buildSlack(settings integrationkinds.Settings) (notifytransport.Transport, error) {
	s, ok := settings.(integrationkinds.SlackSettings)
	if !ok {
		return nil, fmt.Errorf("slack: unexpected settings type %T", settings)
	}
	timeout, err := xtime.ParseTimeout(s.Timeout)
	if err != nil {
		return nil, fmt.Errorf("slack: %w", err)
	}
	return slacktransport.New(slacktransport.Params{
		BotToken: s.BotToken,
		APIURL:   s.APIURL,
		Timeout:  timeout,
	}), nil
}

func buildTelegram(settings integrationkinds.Settings) (notifytransport.Transport, error) {
	s, ok := settings.(integrationkinds.TelegramSettings)
	if !ok {
		return nil, fmt.Errorf("telegram: unexpected settings type %T", settings)
	}
	timeout, err := xtime.ParseTimeout(s.Timeout)
	if err != nil {
		return nil, fmt.Errorf("telegram: %w", err)
	}
	return telegramtransport.New(telegramtransport.Params{
		BotToken: s.BotToken,
		APIURL:   s.APIURL,
		Timeout:  timeout,
	})
}

func buildEmail(settings integrationkinds.Settings) (notifytransport.Transport, error) {
	s, ok := settings.(integrationkinds.EmailSettings)
	if !ok {
		return nil, fmt.Errorf("email: unexpected settings type %T", settings)
	}
	timeout, err := xtime.ParseTimeout(s.Timeout)
	if err != nil {
		return nil, fmt.Errorf("email: %w", err)
	}
	return emailtransport.New(emailtransport.Params{
		Host:      s.Host,
		Port:      s.Port,
		Username:  s.Username,
		Password:  s.Password,
		From:      s.From,
		ReplyTo:   s.ReplyTo,
		TLSPolicy: s.TLSPolicy,
		Timeout:   timeout,
	})
}

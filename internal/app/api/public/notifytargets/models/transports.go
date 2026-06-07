package apimodels

import (
	"strings"

	"github.com/ruko1202/maintmode/internal/entity"
)

// Transport describes a single supported notification transport for the channel-create UI
type Transport struct {
	ID    string `json:"id" example:"slack"`
	Title string `json:"title" example:"Slack"`
}

// TransportsResponse is the payload of GET /api/v1/notifications/transports.
// Wrapping the slice in an object leaves room for future fields without a
// breaking change.
type TransportsResponse struct {
	Transports []*Transport `json:"transports"`
}

// SupportedTransports is the static catalog of transports a channel can be
// created on. It is the single source the GET /transports handler returns, so
// the handler response and tests agree on one definition. Every id must be a
// transport accepted by entity.NotifyTransport.IsValid.
var SupportedTransports = []*Transport{
	{
		ID:    string(entity.NotifyTransportSlack),
		Title: strings.ToTitle(string(entity.NotifyTransportSlack)),
	},
	{
		ID:    string(entity.NotifyTransportTelegram),
		Title: strings.ToTitle(string(entity.NotifyTransportTelegram)),
	},
}

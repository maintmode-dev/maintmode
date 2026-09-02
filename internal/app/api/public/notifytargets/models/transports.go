package apimodels

import (
	"github.com/samber/lo"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/ruko1202/maintmode/internal/entity"
)

// Transport describes a single supported notification transport for the channel-create UI
type Transport struct {
	ID    string `json:"id" example:"slack"`
	Title string `json:"title" example:"Slack"`
	// TransportStatus reports whether the integration backing the transport can
	// deliver right now (ok | disabled | not_configured), so the channel-create
	// form can flag transports that would silently not deliver. The
	// catalog stays advisory: creating a channel on a disabled/unconfigured
	// transport is still allowed.
	TransportStatus TransportStatus `json:"transport_status" example:"ok"`
}

// TransportsResponse is the payload of GET /api/v1/notifications/transports.
// Wrapping the slice in an object leaves room for future fields without a
// breaking change.
type TransportsResponse struct {
	Transports []*Transport `json:"transports"`
}

// ToTransportsResponse projects the static catalog with each transport's
// per-request registry status. It copies the entries — SupportedTransports is
// shared package state and must not be mutated.
func ToTransportsResponse(index TransportStatusIndex) TransportsResponse {
	transports := make([]*Transport, 0, len(SupportedTransports))
	for _, tr := range SupportedTransports {
		withStatus := *tr
		withStatus.TransportStatus = index.StatusFor(entity.NotifyTransport(tr.ID))
		transports = append(transports, &withStatus)
	}
	return TransportsResponse{Transports: transports}
}

// SupportedTransports is the static catalog of transports a channel can be
// created on. It is the single source the GET /transports handler returns, so
// the handler response and tests agree on one definition. Every id must be a
// transport accepted by entity.NotifyTransport.IsValid.
var SupportedTransports = []*Transport{
	{
		ID:    string(entity.NotifyTransportSlack),
		Title: capitalize(string(entity.NotifyTransportSlack)),
	},
	{
		ID:    string(entity.NotifyTransportTelegram),
		Title: capitalize(string(entity.NotifyTransportTelegram)),
	},
}

// SupportedTransportNames returns the catalog ids as transports, for handlers
// that need the per-transport status of the whole catalog.
func SupportedTransportNames() []entity.NotifyTransport {
	return lo.Map(SupportedTransports, func(item *Transport, _ int) entity.NotifyTransport {
		return entity.NotifyTransport(item.ID)
	})
}

func capitalize(s string) string {
	caser := cases.Title(language.Und)

	return caser.String(s)
}

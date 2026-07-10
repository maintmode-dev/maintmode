package apimodels

import (
	"github.com/ruko1202/maintmode/internal/entity"
)

// TransportStatus reports whether the integration backing a transport can
// deliver right now. It is a read-model signal only (RUK-198, вариант В): the
// coupling between channels and the integration registry stays weak — channel
// create/update never validates against the registry, and the dispatch path
// keeps its best-effort drop on a disabled integration. The FE uses this to
// highlight channels/transports that will silently not deliver.
type TransportStatus string

const (
	// TransportStatusOK means an enabled integration exists for the transport.
	TransportStatusOK TransportStatus = "ok"
	// TransportStatusDisabled means the integration exists but is switched off.
	TransportStatusDisabled TransportStatus = "disabled"
	// TransportStatusNotConfigured means no integration is registered for the
	// transport at all.
	TransportStatusNotConfigured TransportStatus = "not_configured"
)

// IntegrationIndex is the kind → enabled view of the integration registry,
// built once per request from the masked (secret-free) listing.
type IntegrationIndex map[string]bool

// NewIntegrationIndex builds the index from the registry's masked listing.
func NewIntegrationIndex(integrations []*entity.MaskedIntegration) IntegrationIndex {
	index := make(IntegrationIndex, len(integrations))
	for _, in := range integrations {
		index[in.Kind] = in.Enabled
	}
	return index
}

// StatusFor maps a channel transport to its delivery-availability status.
// Domain junction: a channel's transport name doubles as the integration kind
// (messenger_channels.transport == integration_settings.kind) — the same
// string equality the delivery read-path relies on in
// internal/services/transportresolver/get.go.
func (ix IntegrationIndex) StatusFor(transport entity.NotifyTransport) TransportStatus {
	enabled, ok := ix[string(transport)]
	switch {
	case !ok:
		return TransportStatusNotConfigured
	case !enabled:
		return TransportStatusDisabled
	default:
		return TransportStatusOK
	}
}

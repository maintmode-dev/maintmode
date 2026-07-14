package apimodels

import (
	"errors"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// TransportStatus reports whether the integration backing a transport can
// deliver right now. It is a read-model signal only (RUK-198, вариант В): the
// coupling between channels and the integration registry stays weak — channel
// create/update never validates against the registry, and the dispatch path
// keeps its best-effort drop on a disabled integration. The FE uses this to
// highlight channels/transports that will silently not deliver.
//
// The status is derived from the SAME resolve call the delivery path makes
// (transportresolver.Get → integration.Settings), so it can never disagree
// with what a real send would do locally (RUK-200). "ok" means "a transport
// resolves and its secrets open" — NOT "delivery is guaranteed": a token
// revoked on the messenger side is only detectable by an actual send.
type TransportStatus string

const (
	// TransportStatusOK means the transport resolves: an enabled integration
	// exists, its secrets decrypt and a delivery client can be built.
	TransportStatusOK TransportStatus = "ok"
	// TransportStatusUnreadable means an enabled integration exists but cannot
	// be resolved locally (rolled-back KEK, corrupt envelope, missing DEK row,
	// unparseable settings) — delivery will fail at dispatch (RUK-200).
	TransportStatusUnreadable TransportStatus = "unreadable"
	// TransportStatusDisabled means the integration exists but is switched off.
	TransportStatusDisabled TransportStatus = "disabled"
	// TransportStatusNotConfigured means no integration is registered for the
	// transport at all.
	TransportStatusNotConfigured TransportStatus = "not_configured"
)

// StatusFromResolve classifies a transportresolver.Get outcome into the
// transport_status enum. Order matters: ErrIntegrationNotConfigured WRAPS
// ErrIntegrationDisabled (to keep the dispatch drop contract), so the finer
// sentinel is checked first. An unclassified error returns ok=false — the
// caller must fail the request loudly (500) rather than invent a status: a
// storage outage is not an integration state.
func StatusFromResolve(err error) (TransportStatus, bool) {
	switch {
	case err == nil:
		return TransportStatusOK, true
	case errors.Is(err, apperr.ErrIntegrationNotConfigured):
		return TransportStatusNotConfigured, true
	case errors.Is(err, apperr.ErrIntegrationUnreadable):
		return TransportStatusUnreadable, true
	case errors.Is(err, apperr.ErrIntegrationDisabled):
		return TransportStatusDisabled, true
	default:
		return "", false
	}
}

// TransportStatusIndex is the per-request transport → status view, built by
// resolving each transport the response needs exactly once.
type TransportStatusIndex map[entity.NotifyTransport]TransportStatus

// StatusFor returns the transport's resolved status. A transport absent from
// the index reads as not_configured — the same value an unknown kind resolves
// to, so a missed lookup degrades to the honest "will not deliver" signal
// rather than a false ok.
func (ix TransportStatusIndex) StatusFor(transport entity.NotifyTransport) TransportStatus {
	if status, ok := ix[transport]; ok {
		return status
	}
	return TransportStatusNotConfigured
}

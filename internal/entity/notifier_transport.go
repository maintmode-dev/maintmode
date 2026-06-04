package entity

import (
	"time"

	"github.com/google/uuid"
)

// NotifyTransport identifies transport in routing config and registries.
type NotifyTransport string

const (
	NotifyTransportStub     NotifyTransport = "stub"
	NotifyTransportSlack    NotifyTransport = "slack"
	NotifyTransportTelegram NotifyTransport = "telegram"
	// NotifyTransportEmail delivers to an email address. Unlike slack/telegram
	// it is a system delivery transport (e.g. user invitations), not a
	// user-subscribable notify channel, so it is intentionally excluded from
	// IsValid below.
	NotifyTransportEmail NotifyTransport = "email"
)

func (t NotifyTransport) IsValid() bool {
	switch t {
	case NotifyTransportSlack,
		NotifyTransportTelegram:
		return true
	default:
		return false
	}
}

// NotifyChannel is a catalog entry. ID is the DB row UUID and the public
// identity used everywhere (GET /channels, subscription validation, archive).
// ArchivedAt is nil for active channels; an archived channel is hidden from the
// default listing but still resolvable by ID.
type NotifyChannel struct {
	ID                 uuid.UUID
	Transport          NotifyTransport
	TransportChannelID string
	Name               string
	Description        string
	ArchivedAt         *time.Time
}

// CreateNotifyChannelCmd is the command to register a new channel in the
// catalog. ID is assigned by the DB, so it is not part of the input.
type CreateNotifyChannelCmd struct {
	Transport          NotifyTransport
	TransportChannelID string
	Name               string
	Description        string
}

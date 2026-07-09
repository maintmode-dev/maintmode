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
		NotifyTransportTelegram,
		NotifyTransportEmail:
		return true
	default:
		return false
	}
}

// NotifyChannel is a catalog entry. ID is the DB row UUID and the public
// identity used everywhere (GET /channels, subscription validation, archive).
// ArchivedAt is nil for active channels; an archived channel is hidden from the
// default listing but still resolvable by ID.
//
// CreatedByUserID / UpdatedByUserID carry the authorship identity (token
// subject) for the detail-page header. They are pointers because pre-existing
// catalog rows have no author and UpdatedByUserID is unset until the first edit;
// read paths resolve a nil id to the "Unknown user" summary.
type NotifyChannel struct {
	ID                 uuid.UUID
	Transport          NotifyTransport
	TransportChannelID string
	Name               string
	Description        string
	CreatedAt          time.Time
	CreatedByUserID    *uuid.UUID
	UpdatedAt          *time.Time
	UpdatedByUserID    *uuid.UUID
	ArchivedAt         *time.Time
}

// CreateNotifyChannelCmd is the command to register a new channel in the
// catalog. ID is assigned by the DB, so it is not part of the input.
// CreatedByUserID is the authenticated author captured from the access token.
type CreateNotifyChannelCmd struct {
	Transport          NotifyTransport
	TransportChannelID string
	Name               string
	Description        string
	CreatedByUserID    uuid.UUID
}

// UpdateNotifyChannelCmd is a partial update of a channel. A nil field is left
// unchanged. Transport is intentionally absent: changing a channel's transport
// would break notification history and existing subscriptions, so a new channel
// must be created instead. UpdatedByUserID is the authenticated editor.
type UpdateNotifyChannelCmd struct {
	ID                 uuid.UUID
	Name               *string
	Description        *string
	TransportChannelID *string
	UpdatedByUserID    uuid.UUID
}

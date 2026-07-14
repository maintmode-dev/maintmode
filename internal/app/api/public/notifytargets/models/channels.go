package apimodels

import (
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
)

// UserSummary is a privacy-safe view of a user (a channel author or editor)
// exposed in API responses. It mirrors the maintenance UserSummary shape so the
// FE renders authorship the same way everywhere. A nil *UserSummary serializes
// as null (unknown/unset author), so clients must handle the null case.
type UserSummary struct {
	ID          uuid.UUID `json:"id" format:"uuid"`
	DisplayName string    `json:"display_name"`
	Email       string    `json:"email"`
}

type Channel struct {
	ID        string `json:"id" format:"uuid"`
	Transport string `json:"transport"`
	// TransportStatus reports whether the integration backing the transport can
	// deliver right now (ok | disabled | not_configured). Always present — the
	// coupling to the registry stays weak, this is the read-model signal that
	// makes silent non-delivery visible (RUK-198).
	TransportStatus    TransportStatus `json:"transport_status" example:"ok"`
	TransportChannelID string          `json:"transport_channel_id"`
	Name               string          `json:"name"`
	Description        string          `json:"description"`
	ArchivedAt         *time.Time      `json:"archived_at,omitempty" format:"date-time"`
	CreatedAt          time.Time       `json:"created_at" format:"date-time"`
	// CreatedBy is the channel author resolved from the auth service. Null for
	// legacy rows with no recorded author; when the id is set but unresolvable
	// (auth down or user removed) it degrades to the "Unknown user" label.
	CreatedBy *UserSummary `json:"created_by"`
	// UpdatedAt is null until the channel is first edited.
	UpdatedAt *time.Time `json:"updated_at,omitempty" format:"date-time"`
	// UpdatedBy is the last editor resolved from the auth service, null until the
	// channel is first edited (degrades to "Unknown user" like CreatedBy).
	UpdatedBy *UserSummary `json:"updated_by"`
}

// CreateChannelRequest is the body of POST /api/v1/notifications/channels.
// The id (a UUID) is assigned by the DB and returned in the response, so it is
// not part of the request.
type CreateChannelRequest struct {
	Transport          string `json:"transport" example:"slack"`
	TransportChannelID string `json:"transport_channel_id" example:"C0123456789"`
	Name               string `json:"name" example:"Alerts"`
	Description        string `json:"description" example:"Ops alerting channel"`
}

// UpdateChannelRequest is the body of PATCH /api/v1/notifications/channels/{id}.
// All fields are optional (partial update); an omitted (nil) field is left
// unchanged. Transport is intentionally absent: it is immutable — switching a
// channel's transport would break notification history and existing
// subscriptions, so a new channel must be created instead. Any transport key in
// the body is silently ignored.
type UpdateChannelRequest struct {
	Name               *string `json:"name" example:"Alerts"`
	Description        *string `json:"description" example:"Ops alerting channel"`
	TransportChannelID *string `json:"transport_channel_id" example:"C0123456789"`
}

// toAPIUserSummary maps a resolved domain user summary to its API shape.
// Nil-safe: a nil summary maps to nil (serialized as null).
func toAPIUserSummary(u *entity.UserSummary) *UserSummary {
	if u == nil {
		return nil
	}

	return &UserSummary{
		ID:          u.ID,
		DisplayName: u.Name,
		Email:       u.Email,
	}
}

// ToChannel maps a domain channel to its API shape. author and editor are the
// authorship summaries resolved from auth (nil when the channel carries no
// corresponding user id, e.g. an unedited channel has no editor). index is the
// per-request integration registry view the transport status is derived from.
func ToChannel(channel *entity.NotifyChannel, author, editor *entity.UserSummary, index TransportStatusIndex) *Channel {
	return &Channel{
		ID:                 channel.ID.String(),
		Transport:          string(channel.Transport),
		TransportStatus:    index.StatusFor(channel.Transport),
		TransportChannelID: channel.TransportChannelID,
		Name:               channel.Name,
		Description:        channel.Description,
		ArchivedAt:         channel.ArchivedAt,
		CreatedAt:          channel.CreatedAt,
		CreatedBy:          toAPIUserSummary(author),
		UpdatedAt:          channel.UpdatedAt,
		UpdatedBy:          toAPIUserSummary(editor),
	}
}

// ChannelsResponse is the payload of GET /api/v1/notifications/channels.
// Wrapping the slice in an object leaves room for future fields
// (pagination, transport metadata) without a breaking change.
type ChannelsResponse struct {
	Channels []*Channel `json:"channels"`
}

// ToChannelsResponse maps the catalog to its API shape, looking up each
// channel's author/editor summary in the pre-resolved index (keyed by user id)
// and each channel's transport status in the integration registry view.
func ToChannelsResponse(channels []*entity.NotifyChannel, summaries map[uuid.UUID]*entity.UserSummary, index TransportStatusIndex) ChannelsResponse {
	return ChannelsResponse{
		Channels: lo.Map(channels, func(item *entity.NotifyChannel, _ int) *Channel {
			return ToChannel(item, lookupSummary(summaries, item.CreatedByUserID), lookupSummary(summaries, item.UpdatedByUserID), index)
		}),
	}
}

// lookupSummary resolves a (possibly nil) user id against the summary index.
func lookupSummary(summaries map[uuid.UUID]*entity.UserSummary, id *uuid.UUID) *entity.UserSummary {
	if id == nil {
		return nil
	}
	return summaries[*id]
}

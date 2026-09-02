package apiauthmodels

import (
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
)

// MeResponse is the authenticated user's profile.
//
// ConnectedProviders lists every OAuth provider the user can sign in with.
// OAuthProvider is kept for backward compatibility and reports the user's
// primary (first-linked) provider, or "unknown" when none are linked.
type MeResponse struct {
	ID                 string   `json:"id"`
	Email              string   `json:"email"`
	DisplayName        string   `json:"display_name"`
	Roles              []string `json:"roles"`
	OAuthProvider      string   `json:"oauth_provider"`
	ConnectedProviders []string `json:"connected_providers"`
	// Timezone is the user's preferred IANA timezone (e.g. "Asia/Nicosia"), or
	// null when not selected — the frontend then falls back to browser auto-detect.
	Timezone *string `json:"timezone"`
	// TelegramTag and SlackTag are the user's messenger handles, returned exactly
	// as entered (a leading "@" only if the user typed one), or null when not set.
	TelegramTag *string `json:"telegram_tag"`
	SlackTag    *string `json:"slack_tag"`
}

func ToAPIMeResponse(u *entity.User, providers []entity.AuthMethod) *MeResponse {
	connected := lo.Map(providers, func(p entity.AuthMethod, _ int) string {
		return string(p)
	})

	return &MeResponse{
		ID:          u.ID.String(),
		Email:       u.Email,
		DisplayName: u.Name,
		Roles: lo.Map(u.Roles, func(item entity.Role, _ int) string {
			return string(item)
		}),
		OAuthProvider:      string(entity.PrimaryAuthMethod(providers)),
		ConnectedProviders: connected,
		Timezone:           u.Timezone,
		TelegramTag:        u.TelegramTag,
		SlackTag:           u.SlackTag,
	}
}

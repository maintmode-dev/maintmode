package apiauthmodels

import (
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
)

// MeResponse is the authenticated user's profile.
// OAuthProvider is hardcoded to "google" for now — Google is the only provider.
// When more providers are supported, store the source on entity.User and return it here.
type MeResponse struct {
	ID            string   `json:"id"`
	Email         string   `json:"email"`
	DisplayName   string   `json:"display_name"`
	Roles         []string `json:"roles"`
	OAuthProvider string   `json:"oauth_provider"`
}

func ToAPIMeResponse(u *entity.User) *MeResponse {
	return &MeResponse{
		ID:          u.ID.String(),
		Email:       u.Email,
		DisplayName: u.Name,
		Roles: lo.Map(u.Roles, func(item entity.Role, _ int) string {
			return string(item)
		}),
		OAuthProvider: string(entity.OAuthProviderGoogle),
	}
}

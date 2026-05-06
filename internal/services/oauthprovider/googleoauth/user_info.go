package googleoauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/utils/xhttp"

	"github.com/ruko1202/maintmode/internal/entity"
)

type googleUserInfoPayload struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// UserInfo fetches the authenticated user's profile from Google.
func (g *Service) UserInfo(ctx context.Context, accessToken string) (*entity.OAuthProviderUserInfo, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.OAuth.Google.UserInfo")
	defer span.End()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.googleUserInfoURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create userinfo request: %w", err)
	}
	xhttp.SetBearerToken(req, accessToken)

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch userinfo: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google userinfo returned status %d", resp.StatusCode)
	}

	var payload googleUserInfoPayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode userinfo: %w", err)
	}

	return &entity.OAuthProviderUserInfo{
		ID:    payload.ID,
		Email: payload.Email,
		Name:  payload.Name,
	}, nil
}

package googleoauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/utils/xhttp"

	"github.com/ruko1202/maintmode/internal/entity"
)

type googleUserInfoPayload struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// UserInfo fetches the authenticated user's profile from Google.
func (p *Service) UserInfo(ctx context.Context, accessToken string) (*entity.OAuthProviderUserInfo, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.OAuth.Google.UserInfo")
	defer span.End()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.googleUserInfoURL, http.NoBody)
	if err != nil {
		xlog.Error(ctx, "failed to create userinfo request", xfield.Error(err))
		return nil, fmt.Errorf("create userinfo request: %w", err)
	}
	xhttp.SetBearerToken(req, accessToken)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		xlog.Error(ctx, "failed to fetch userinfo", xfield.Error(err))
		return nil, fmt.Errorf("fetch userinfo: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("google userinfo returned status %d", resp.StatusCode)
		xlog.Error(ctx, "failed to fetch userinfo", xfield.Error(err))
		return nil, err
	}

	var payload googleUserInfoPayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		xlog.Error(ctx, "failed to decode userinfo", xfield.Error(err))
		return nil, fmt.Errorf("decode userinfo: %w", err)
	}

	return &entity.OAuthProviderUserInfo{
		ID:    payload.ID,
		Email: payload.Email,
		Name:  payload.Name,
	}, nil
}

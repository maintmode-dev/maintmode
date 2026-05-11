package googleoauth

import (
	"context"

	"github.com/ruko1202/xlog"
	"golang.org/x/oauth2"
)

// AuthCodeURL returns the Google consent screen URL for the given state.
func (p *Service) AuthCodeURL(ctx context.Context, state string) string {
	_, span := xlog.WithOperationSpan(ctx, "service.OAuth.Google.AuthCodeURL")
	defer span.End()

	return p.oauth2Config.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

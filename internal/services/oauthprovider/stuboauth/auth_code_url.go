package stuboauth

import (
	"context"
	"net/url"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
)

// AuthCodeURL returns the Google consent screen URL for the given state.
func (g *Service) AuthCodeURL(ctx context.Context, state string) string {
	_, span := xlog.WithOperationSpan(ctx, "service.OAuth.Stub.AuthCodeURL")
	defer span.End() //nolint:gocritic

	u, err := url.Parse(g.authRedirectURL)
	if err != nil {
		xlog.Error(ctx, "failed to parse auth redirect url", xfield.Error(err))
		return ""
	}

	query := u.Query()
	query.Add("state", state)
	query.Add("code", "oauth-stub-code")

	u.RawQuery = query.Encode()

	return u.String()
}

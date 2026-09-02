package googleoauth

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xjwt"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

// Service verifies Google-issued ID tokens. The authorization-code exchange
// lives in the BFF (maintmode-ui), so this provider never talks to Google's
// token endpoint: no client secret, no redirect URL, no HTTP client for the
// token or userinfo endpoints. The one thing it needs from the OAuth client is
// the client_id, which every ID token must carry as its audience.
//
// The verifier state is held flat, mirroring jwtverifier.Service: cfg plus the
// keyfunc it feeds, plus the timestamp its refresh-failure callback writes.
type Service struct {
	clientID            string
	cfg                 config.JWTVerifierConfig
	keyfunc             keyfunc.Keyfunc
	lastRefreshFailedAt atomic.Int64
}

// NewProvider creates a GoogleOAuth provider.
func NewProvider(ctx context.Context, cfg *config.GoogleOauthProvider) (*Service, error) {
	s := &Service{
		clientID: cfg.ClientID,
		cfg:      cfg.JWTVerify,
	}

	// Bound to s before the keyfunc exists: NewKeyFunc installs this as the
	// refresh-failure callback and may invoke it during the first fetch.
	kf, err := xjwt.NewKeyFunc(ctx, cfg.JWTVerify, s.updateLastRefreshFailedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to init google verifier: %w", err)
	}
	s.keyfunc = kf

	return s, nil
}

func (s *Service) MethodID() entity.AuthMethod {
	return entity.AuthMethodGoogle
}

func (s *Service) updateLastRefreshFailedAt(ctx context.Context) {
	xlog.Info(ctx, "update last refresh failed at")
	s.lastRefreshFailedAt.Store(xtime.UTCNow().UnixNano())
}

func (s *Service) LastRefreshFailedAt(_ context.Context) int64 {
	return s.lastRefreshFailedAt.Load()
}

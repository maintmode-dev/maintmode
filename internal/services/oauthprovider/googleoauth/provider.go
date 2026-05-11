package googleoauth

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/ruko1202/xlog"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xhttp"
	"github.com/ruko1202/maintmode/internal/utils/xjwt"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

// Service implements repository.GoogleOAuthProvider using golang.org/x/oauth2.
type Service struct {
	oauth2Config      *oauth2.Config
	googleUserInfoURL string
	httpClient        *http.Client
	jwtVerifier       *googleVerifier
}

// NewProvider creates a GoogleOAuth provider.
func NewProvider(ctx context.Context, cfg *config.GoogleOauthProvider) (*Service, error) {
	s := &Service{
		oauth2Config: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Scopes:       cfg.Scopes,
			Endpoint:     google.Endpoint,
		},
		googleUserInfoURL: cfg.GoogleUserInfoURL,
		httpClient:        xhttp.NewClient(),
	}

	verifier, err := newGoogleVerifier(ctx, cfg.JWTVerify)
	if err != nil {
		return nil, fmt.Errorf("failed to init google verifier: %w", err)
	}
	s.jwtVerifier = verifier

	return s, nil
}

func (p *Service) ProviderID() entity.OAuthProvider {
	return entity.OAuthProviderGoogle
}

type googleVerifier struct {
	cfg                 config.JWTVerifierConfig
	keyfunc             keyfunc.Keyfunc
	lastRefreshFailedAt atomic.Int64
}

func newGoogleVerifier(ctx context.Context, cfg config.JWTVerifierConfig) (*googleVerifier, error) {
	v := &googleVerifier{
		cfg: cfg,
	}

	kf, err := xjwt.NewKeyFunc(ctx, cfg, v.updateLastRefreshFailedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to init google keyfunc: %w", err)
	}
	v.keyfunc = kf

	return v, nil
}

func (s *googleVerifier) updateLastRefreshFailedAt(ctx context.Context) {
	xlog.Info(ctx, "update last refresh failed at")
	s.lastRefreshFailedAt.Store(xtime.UTCNow().UnixNano())
}

func (s *googleVerifier) LastRefreshFailedAt(_ context.Context) int64 {
	return s.lastRefreshFailedAt.Load()
}

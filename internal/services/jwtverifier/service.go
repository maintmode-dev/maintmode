package jwtverifier

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"

	"github.com/MicahParks/jwkset"
	"github.com/MicahParks/keyfunc/v3"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"golang.org/x/time/rate"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

type Service struct {
	cfg                 config.JWTVerifierConfig
	keyfunc             keyfunc.Keyfunc
	lastRefreshFailedAt atomic.Int64
}

func NewService(ctx context.Context, cfg config.JWTVerifierConfig) (*Service, error) {
	s := &Service{
		cfg: cfg,
	}

	if err := s.initKeyFunc(ctx); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Service) initKeyFunc(ctx context.Context) error {
	storage, err := jwkset.NewStorageFromHTTP(s.cfg.JWKSURL, jwkset.HTTPClientStorageOptions{
		Client:                    &http.Client{Timeout: s.cfg.JWKSHTTPTimeout},
		Ctx:                       ctx,
		HTTPTimeout:               s.cfg.JWKSHTTPTimeout,
		NoErrorReturnFirstHTTPReq: true,
		RefreshInterval:           s.cfg.JWKSRefreshInterval,
		RefreshErrorHandler: func(ctx context.Context, err error) {
			s.updateLastRefreshFailedAt()
			xlog.Error(ctx, "refresh jwks failed", xfield.String("jwks_url", s.cfg.JWKSURL), xfield.Error(err))
		},
	})
	if err != nil {
		return fmt.Errorf("create jwks storage: %w", err)
	}

	jwksClient, err := jwkset.NewHTTPClient(jwkset.HTTPClientOptions{
		HTTPURLs: map[string]jwkset.Storage{
			s.cfg.JWKSURL: storage,
		},
		PrioritizeHTTP:    true,
		RateLimitWaitMax:  s.cfg.JWKSUnknownKIDWaitMax,
		RefreshUnknownKID: rate.NewLimiter(rate.Every(s.cfg.JWKSUnknownKIDRefreshRate), 1),
	})
	if err != nil {
		return fmt.Errorf("create jwks client: %w", err)
	}

	keyFunc, err := keyfunc.New(keyfunc.Options{
		Ctx:     ctx,
		Storage: jwksClient,
	})
	if err != nil {
		return fmt.Errorf("create jwt keyfunc: %w", err)
	}

	s.keyfunc = keyFunc
	return nil
}

func (s *Service) LastRefreshFailedAt() int64 {
	return s.lastRefreshFailedAt.Load()
}

func (s *Service) updateLastRefreshFailedAt() {
	s.lastRefreshFailedAt.Store(xtime.UTCNow().UnixNano())
}

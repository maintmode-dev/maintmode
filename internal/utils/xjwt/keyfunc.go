package xjwt

import (
	"context"
	"fmt"

	"github.com/MicahParks/jwkset"
	"github.com/MicahParks/keyfunc/v3"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"golang.org/x/time/rate"

	"github.com/ruko1202/xhttp/client"

	"github.com/ruko1202/maintmode/internal/utils/xsanitize"

	"github.com/ruko1202/maintmode/internal/config"
)

func NewKeyFunc(
	ctx context.Context,
	cfg config.JWTVerifierConfig,
	updateLastRefreshFailedAtFunc func(ctx context.Context),
) (keyfunc.Keyfunc, error) {
	storage, err := jwkset.NewStorageFromHTTP(cfg.JWKSURL, jwkset.HTTPClientStorageOptions{
		Client: client.NewClient(
			client.WithTimeout(cfg.JWKSHTTPTimeout),
			client.WithSanitizer(xsanitize.New()),
		),
		Ctx:                       ctx,
		HTTPTimeout:               cfg.JWKSHTTPTimeout,
		NoErrorReturnFirstHTTPReq: true,
		RefreshInterval:           cfg.JWKSRefreshInterval,
		RefreshErrorHandler: func(ctx context.Context, err error) {
			updateLastRefreshFailedAtFunc(ctx)
			xlog.Error(ctx, "refresh jwks failed", xfield.String("jwks_url", cfg.JWKSURL), xfield.Error(err))
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create jwks storage: %w", err)
	}

	jwksClient, err := jwkset.NewHTTPClient(jwkset.HTTPClientOptions{
		HTTPURLs: map[string]jwkset.Storage{
			cfg.JWKSURL: storage,
		},
		PrioritizeHTTP:    true,
		RateLimitWaitMax:  cfg.JWKSUnknownKIDWaitMax,
		RefreshUnknownKID: rate.NewLimiter(rate.Every(cfg.JWKSUnknownKIDRefreshRate), 1),
	})
	if err != nil {
		return nil, fmt.Errorf("create jwks client: %w", err)
	}

	keyFunc, err := keyfunc.New(keyfunc.Options{
		Ctx:     ctx,
		Storage: jwksClient,
	})
	if err != nil {
		return nil, fmt.Errorf("create jwt keyfunc: %w", err)
	}

	return keyFunc, nil
}

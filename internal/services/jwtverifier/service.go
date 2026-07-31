package jwtverifier

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/utils/xjwt"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

type Service struct {
	cfg                 config.JWTVerifierConfig
	keyfunc             keyfunc.Keyfunc
	lastRefreshFailedAt atomic.Int64
}

// NewService builds a verifier over the issuer's own public key. The signing key
// already lives in this process, so jwtCfg is read directly instead of fetching
// the public half over HTTP from ourselves — that round trip only bought a
// startup race between the verifier and the HTTP listener.
func NewService(ctx context.Context, cfg config.JWTVerifierConfig, jwtCfg config.JWT) (*Service, error) {
	s := &Service{
		cfg: cfg,
	}

	// The error must propagate: NewLocalKeyFunc panics inside the jwkset library
	// on a nil key rather than reporting it.
	pub, err := jwtCfg.PublicKey()
	if err != nil {
		return nil, fmt.Errorf("failed to derive jwt public key: %w", err)
	}

	kf, err := xjwt.NewLocalKeyFunc(ctx, pub, jwtCfg.Kid)
	if err != nil {
		return nil, err
	}
	s.keyfunc = kf

	return s, nil
}

func (s *Service) LastRefreshFailedAt(_ context.Context) int64 {
	return s.lastRefreshFailedAt.Load()
}

// updateLastRefreshFailedAt has no caller since the verifier stopped fetching its
// key over HTTP: it was the refresh-failure callback of the network keyfunc. It is
// kept, rather than deleted, as the only writer of lastRefreshFailedAt, which
// authUnavailable still reads — so the ErrAuthUnavailable branch keeps a meaning if
// a genuinely remote key source is ever added back.
//
//nolint:unused // retained as the counterpart of authUnavailable's read; see above.
func (s *Service) updateLastRefreshFailedAt(ctx context.Context) {
	xlog.Info(ctx, "update last refresh failed at")
	s.lastRefreshFailedAt.Store(xtime.UTCNow().UnixNano())
}

package jwtverifier

import (
	"context"
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

func NewService(ctx context.Context, cfg config.JWTVerifierConfig) (*Service, error) {
	s := &Service{
		cfg: cfg,
	}

	kf, err := xjwt.NewKeyFunc(ctx, cfg, s.updateLastRefreshFailedAt)
	if err != nil {
		return nil, err
	}
	s.keyfunc = kf

	return s, nil
}

func (s *Service) LastRefreshFailedAt(_ context.Context) int64 {
	return s.lastRefreshFailedAt.Load()
}

func (s *Service) updateLastRefreshFailedAt(ctx context.Context) {
	xlog.Info(ctx, "update last refresh failed at")
	s.lastRefreshFailedAt.Store(xtime.UTCNow().UnixNano())
}

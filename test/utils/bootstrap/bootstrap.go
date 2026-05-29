package testbootstraputils

import (
	"context"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/app/bootstrap"
	"github.com/ruko1202/maintmode/internal/config"
)

func InitStores(
	cfg *config.AppConfig,
	db *sqlx.DB,
) *bootstrap.Stores {
	stores, err := bootstrap.NewStores(cfg, db)
	if err != nil {
		panic(err)
	}

	return stores
}

func InitGateways(
	cfg *config.AppConfig,
) *bootstrap.Gateways {
	gateways, err := bootstrap.NewGateways(cfg)
	if err != nil {
		panic(err)
	}

	return gateways
}

func InitServices(
	ctx context.Context,
	db *sqlx.DB,
	cfg *config.AppConfig,
) *bootstrap.Services {
	services, err := bootstrap.NewServices(
		ctx,
		cfg,
		InitStores(cfg, db),
		InitGateways(cfg),
	)
	if err != nil {
		panic(err)
	}

	return services
}

func InitServicesT(
	ctx context.Context,
	t *testing.T,
	db *sqlx.DB,
	cfg *config.AppConfig,
) *bootstrap.Services {
	t.Helper()

	var services *bootstrap.Services

	require.NotPanics(t, func() {
		services = InitServices(ctx, db, cfg)
	}, "InitServices failed")

	return services
}

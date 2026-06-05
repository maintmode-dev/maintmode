package testbootstraputils

import (
	"context"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/app/bootstrap"
	"github.com/ruko1202/maintmode/internal/config"
)

func InitStores(
	db *sqlx.DB,
) *bootstrap.Stores {
	stores, err := bootstrap.NewStores(db)
	if err != nil {
		panic(err)
	}

	return stores
}

func InitServicesT(
	ctx context.Context,
	t *testing.T,
	db *sqlx.DB,
	cfg *config.AppConfig,
) *bootstrap.Services {
	t.Helper()

	services, _ := InitServicesWithMocks(ctx, t, db, cfg)
	return services
}

// InitServicesWithMocks builds services with the mock set bound to the test's
// controller and returns the mocks so a test can append specific EXPECT()
// overrides (gomock matches the most recent matching expectation first). The
// auth S2S gateway is always the mock, so tests run without a live auth service.
func InitServicesWithMocks(
	ctx context.Context,
	t *testing.T,
	db *sqlx.DB,
	cfg *config.AppConfig,
) (*bootstrap.Services, *Mocks) {
	t.Helper()

	gateways, err := bootstrap.NewGateways(cfg)
	require.NoError(t, err)

	mocks := NewMocks(gomock.NewController(t))
	gateways.Auth = mocks.Auth

	services, err := bootstrap.NewServices(ctx, cfg, InitStores(db), gateways)
	require.NoError(t, err)

	return services, mocks
}

package license

import (
	"context"
	"errors"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/apperr"

	"github.com/ruko1202/maintmode/internal/entity"
)

func TestLicense_ReadThroughCache(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("reads from the store on a cache miss, then serves from cache", func(t *testing.T) {
		t.Parallel()
		svc, m := newServiceWithMocks(t)

		stored := &entity.License{Status: entity.LicenseStatusBlocked, SeatsPurchased: lo.ToPtr(int64(5))}
		// Store.Get is consulted exactly once: the first call fills the cache,
		// the second is served from it (Times(1) fails if the cache is bypassed).
		m.store.EXPECT().Get(gomock.Any()).Return(stored, nil).Times(1)

		require.Equal(t, stored, svc.License(ctx))
		require.Equal(t, stored, svc.License(ctx))
	})

	t.Run("fail-open: a store error yields a non-blocking active license", func(t *testing.T) {
		t.Parallel()
		svc, m := newServiceWithMocks(t)

		m.store.EXPECT().Get(gomock.Any()).Return(nil, errors.New("db down"))

		lic := svc.License(ctx)
		require.NotNil(t, lic)
		require.False(t, lic.Blocked(), "a read failure must never block the instance")
	})

	t.Run("fail-open: an empty cache (fresh instance) also does not block", func(t *testing.T) {
		t.Parallel()
		svc, m := newServiceWithMocks(t)

		m.store.EXPECT().Get(gomock.Any()).Return(nil, apperr.ErrLicenseCacheEmpty)

		require.False(t, svc.License(ctx).Blocked())
	})
}

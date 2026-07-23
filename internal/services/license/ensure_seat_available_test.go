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

// runInTx invokes EnsureSeatAvailable inside a real transaction so the advisory
// lock has an active tx to attach to (mirrors how every real call-site runs the
// guard inside the caller's mutation tx).
func runInTx(t *testing.T, svc *Service) error {
	t.Helper()
	return txManager().WithinTx(context.Background(), func(ctx context.Context) error {
		return svc.EnsureSeatAvailable(ctx)
	})
}

func TestService_EnsureSeatAvailable(t *testing.T) {
	seatRoles := [][]entity.Role{{entity.RoleAdmin}, {entity.RoleReviewer}, {entity.RoleEditor}}

	t.Run("no license: unlimited, guard is a no-op and never touches stores", func(t *testing.T) {
		svc, m := newServiceWithMocks(t)
		m.store.EXPECT().Get(gomock.Any()).Return(nil, apperr.ErrLicenseCacheEmpty)
		// No ListActiveRoles / ListPendingRoles expectation: the guard must return
		// before counting.
		require.NoError(t, svc.EnsureSeatAvailable(context.Background()))
	})

	t.Run("nil seats_purchased: no reported cap yet, unlimited", func(t *testing.T) {
		svc, m := newServiceWithMocks(t)
		m.store.EXPECT().Get(gomock.Any()).Return(&entity.License{Status: entity.LicenseStatusActive}, nil)
		require.NoError(t, svc.EnsureSeatAvailable(context.Background()))
	})

	t.Run("room left: three occupied under a cap of five passes", func(t *testing.T) {
		svc, m := newServiceWithMocks(t)
		m.store.EXPECT().Get(gomock.Any()).Return(licenseWithCap(5), nil)
		m.users.EXPECT().ListActiveRoles(gomock.Any()).Return(seatRoles, nil)
		m.invitations.EXPECT().ListPendingRoles(gomock.Any()).Return(nil, nil)

		require.NoError(t, runInTx(t, svc))
	})

	t.Run("last seat fills exactly: occupied+1 == cap passes (off-by-one)", func(t *testing.T) {
		svc, m := newServiceWithMocks(t)
		m.store.EXPECT().Get(gomock.Any()).Return(licenseWithCap(4), nil)
		// 3 occupied, cap 4: occupied+1 = 4 <= 4, the last seat is grantable.
		m.users.EXPECT().ListActiveRoles(gomock.Any()).Return(seatRoles, nil)
		m.invitations.EXPECT().ListPendingRoles(gomock.Any()).Return(nil, nil)

		require.NoError(t, runInTx(t, svc))
	})

	t.Run("at cap: occupied == cap rejects with ErrSeatsLimitExceeded", func(t *testing.T) {
		svc, m := newServiceWithMocks(t)
		m.store.EXPECT().Get(gomock.Any()).Return(licenseWithCap(3), nil)
		// 3 occupied, cap 3: occupied+1 = 4 > 3, no room for a new seat.
		m.users.EXPECT().ListActiveRoles(gomock.Any()).Return(seatRoles, nil)
		m.invitations.EXPECT().ListPendingRoles(gomock.Any()).Return(nil, nil)

		err := runInTx(t, svc)
		require.ErrorIs(t, err, apperr.ErrSeatsLimitExceeded)
	})

	t.Run("pending seat-invites count toward the cap", func(t *testing.T) {
		svc, m := newServiceWithMocks(t)
		m.store.EXPECT().Get(gomock.Any()).Return(licenseWithCap(3), nil)
		// 2 active + 1 pending seat = 3 occupied, cap 3: rejected. Proves pending
		// invites are counted (would pass if only Active were summed).
		m.users.EXPECT().ListActiveRoles(gomock.Any()).Return([][]entity.Role{{entity.RoleAdmin}, {entity.RoleEditor}}, nil)
		m.invitations.EXPECT().ListPendingRoles(gomock.Any()).Return([][]entity.Role{{entity.RoleReviewer}}, nil)

		err := runInTx(t, svc)
		require.ErrorIs(t, err, apperr.ErrSeatsLimitExceeded)
	})

	t.Run("guests never occupy a seat", func(t *testing.T) {
		svc, m := newServiceWithMocks(t)
		m.store.EXPECT().Get(gomock.Any()).Return(licenseWithCap(1), nil)
		// Many guests, zero seats occupied, cap 1: a seat is grantable.
		m.users.EXPECT().ListActiveRoles(gomock.Any()).Return([][]entity.Role{{entity.RoleGuest}, {entity.RoleGuest}}, nil)
		m.invitations.EXPECT().ListPendingRoles(gomock.Any()).Return([][]entity.Role{{entity.RoleGuest}}, nil)

		require.NoError(t, runInTx(t, svc))
	})

	t.Run("store error fails safe: surfaces as an error, not a false cap breach", func(t *testing.T) {
		svc, m := newServiceWithMocks(t)
		m.store.EXPECT().Get(gomock.Any()).Return(licenseWithCap(10), nil)
		storeErr := errors.New("db down")
		m.users.EXPECT().ListActiveRoles(gomock.Any()).Return(nil, storeErr)
		// ListPendingRoles is not reached once the active count fails.

		err := runInTx(t, svc)
		require.Error(t, err)
		require.NotErrorIs(t, err, apperr.ErrSeatsLimitExceeded, "a store error must not read as a cap breach")
	})
}

func TestNoop_EnsureSeatAvailable(t *testing.T) {
	t.Parallel()
	require.NoError(t, NewNoop().EnsureSeatAvailable(context.Background()))
}

func licenseWithCap(n int64) *entity.License {
	return &entity.License{Status: entity.LicenseStatusActive, SeatsPurchased: lo.ToPtr(n)}
}

package license

import (
	"context"
	"errors"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/entity"
)

func expectReportCollection(m serviceMocks) {
	m.users.EXPECT().ListActiveRoles(gomock.Any()).Return([][]entity.Role{{entity.RoleAdmin}}, nil)
	m.invitations.EXPECT().ListPendingRoles(gomock.Any()).Return(nil, nil)
	m.audit.EXPECT().LastActivityAt(gomock.Any()).Return(nil, nil)
}

func TestRefresh_SendsReportAndUpsertsLicense(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc, m := newServiceWithMocks(t)
	expectReportCollection(m)

	granted := &entity.License{Status: entity.LicenseStatusActive, SeatsPurchased: lo.ToPtr(int64(7))}
	m.client.EXPECT().Send(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, report *entity.HeartbeatReport) (*entity.License, error) {
			require.Equal(t, "v-test", report.Version)
			require.EqualValues(t, 1, report.SeatsUsed.SeatsUsed())
			return granted, nil
		},
	)
	m.store.EXPECT().Upsert(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, lic *entity.License) error {
			require.Equal(t, entity.LicenseStatusActive, lic.Status)
			require.NotNil(t, lic.FetchedAt, "FetchedAt must be stamped before persisting")
			require.False(t, lic.FetchedAt.IsZero(), "FetchedAt must be a real time")
			return nil
		},
	)

	require.NoError(t, svc.Refresh(ctx))
}

// Console-side failure is fail-open: log and return nil (the tick is done), the
// cached license in the DB is untouched.
func TestRefresh_ConsoleFailureIsFailOpen(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc, m := newServiceWithMocks(t)
	expectReportCollection(m)
	m.client.EXPECT().Send(gomock.Any(), gomock.Any()).Return(nil, errors.New("console down"))
	// No Upsert expected: a failed heartbeat must not overwrite the cache.

	require.NoError(t, svc.Refresh(ctx))
}

// Local failures (collecting the report) are real errors — the next tick retries.
func TestRefresh_LocalFailureIsReturned(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc, m := newServiceWithMocks(t)
	wantErr := errors.New("db down")
	m.users.EXPECT().ListActiveRoles(gomock.Any()).Return(nil, wantErr)

	require.ErrorIs(t, svc.Refresh(ctx), wantErr)
}

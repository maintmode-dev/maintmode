package license

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

func TestService_SeatsUsage(t *testing.T) {
	t.Run("counts active and pending under a reported cap", func(t *testing.T) {
		svc, m := newServiceWithMocks(t)
		m.store.EXPECT().Get(gomock.Any()).Return(licenseWithCap(5), nil)
		m.users.EXPECT().ListActiveRoles(gomock.Any()).Return([][]entity.Role{
			{entity.RoleAdmin}, {entity.RoleReviewer}, {entity.RoleEditor}, {entity.RoleEditor},
		}, nil)
		m.invitations.EXPECT().ListPendingRoles(gomock.Any()).Return([][]entity.Role{{entity.RoleEditor}}, nil)

		usage, lic, err := svc.SeatsUsage(context.Background())
		require.NoError(t, err)

		require.EqualValues(t, 4, usage.SeatsUsed())
		require.EqualValues(t, 1, usage.SeatsPending())
		require.EqualValues(t, 5, usage.SeatsOccupied())
		require.NotNil(t, lic)
		require.NotNil(t, lic.SeatsPurchased, "a reported cap is not unlimited")
		require.EqualValues(t, 5, *lic.SeatsPurchased)
	})

	t.Run("guests are counted in no seat bucket", func(t *testing.T) {
		svc, m := newServiceWithMocks(t)
		m.store.EXPECT().Get(gomock.Any()).Return(licenseWithCap(5), nil)
		m.users.EXPECT().ListActiveRoles(gomock.Any()).Return([][]entity.Role{
			{entity.RoleGuest}, {entity.RoleGuest}, {entity.RoleEditor},
		}, nil)
		m.invitations.EXPECT().ListPendingRoles(gomock.Any()).Return([][]entity.Role{{entity.RoleGuest}}, nil)

		usage, _, err := svc.SeatsUsage(context.Background())
		require.NoError(t, err)

		require.EqualValues(t, 1, usage.SeatsUsed())
		require.Zero(t, usage.SeatsPending())
		require.EqualValues(t, 1, usage.SeatsOccupied())
	})

	t.Run("unreadable license means no cap, counts are still collected", func(t *testing.T) {
		svc, m := newServiceWithMocks(t)
		// License() fails open with a synthetic active license carrying no cap, so
		// an unreadable license_cache row reads as unlimited rather than as an
		// error: the counts are honest, only the ceiling is unknown.
		m.store.EXPECT().Get(gomock.Any()).Return(nil, apperr.ErrLicenseCacheEmpty)
		m.users.EXPECT().ListActiveRoles(gomock.Any()).Return([][]entity.Role{{entity.RoleAdmin}}, nil)
		m.invitations.EXPECT().ListPendingRoles(gomock.Any()).Return(nil, nil)

		usage, lic, err := svc.SeatsUsage(context.Background())
		require.NoError(t, err)
		require.EqualValues(t, 1, usage.SeatsUsed())
		require.NotNil(t, lic)
		require.Nil(t, lic.SeatsPurchased, "no cap known")
	})

	t.Run("Console has not reported a cap yet: unlimited", func(t *testing.T) {
		svc, m := newServiceWithMocks(t)
		m.store.EXPECT().Get(gomock.Any()).Return(&entity.License{Status: entity.LicenseStatusActive}, nil)
		m.users.EXPECT().ListActiveRoles(gomock.Any()).Return([][]entity.Role{{entity.RoleAdmin}}, nil)
		m.invitations.EXPECT().ListPendingRoles(gomock.Any()).Return(nil, nil)

		_, lic, err := svc.SeatsUsage(context.Background())
		require.NoError(t, err)
		require.NotNil(t, lic)
		require.Nil(t, lic.SeatsPurchased)
	})

	t.Run("overage is reported as is, not an error", func(t *testing.T) {
		svc, m := newServiceWithMocks(t)
		m.store.EXPECT().Get(gomock.Any()).Return(licenseWithCap(5), nil)
		m.users.EXPECT().ListActiveRoles(gomock.Any()).Return([][]entity.Role{
			{entity.RoleAdmin}, {entity.RoleAdmin}, {entity.RoleReviewer},
			{entity.RoleEditor}, {entity.RoleEditor}, {entity.RoleEditor},
		}, nil)
		m.invitations.EXPECT().ListPendingRoles(gomock.Any()).Return([][]entity.Role{{entity.RoleEditor}}, nil)

		usage, lic, err := svc.SeatsUsage(context.Background())
		require.NoError(t, err)
		require.EqualValues(t, 7, usage.SeatsOccupied())
		require.EqualValues(t, 5, *lic.SeatsPurchased)
	})

	t.Run("active roles unreadable: error up, not a zeroed report", func(t *testing.T) {
		svc, m := newServiceWithMocks(t)
		storeErr := errors.New("db down")
		m.users.EXPECT().ListActiveRoles(gomock.Any()).Return(nil, storeErr)
		// The license is never read once the counts fail.

		_, _, err := svc.SeatsUsage(context.Background())
		require.ErrorIs(t, err, storeErr)
	})

	t.Run("pending roles unreadable: error up, not a zeroed report", func(t *testing.T) {
		svc, m := newServiceWithMocks(t)
		storeErr := errors.New("db down")
		m.users.EXPECT().ListActiveRoles(gomock.Any()).Return([][]entity.Role{{entity.RoleAdmin}}, nil)
		m.invitations.EXPECT().ListPendingRoles(gomock.Any()).Return(nil, storeErr)

		usage, lic, err := svc.SeatsUsage(context.Background())
		require.ErrorIs(t, err, storeErr)
		require.Equal(t, entity.SeatUsage{}, usage, "a partial count would read as free seats")
		require.Nil(t, lic)
	})
}

// TestService_SeatsUsageMatchesEnforcement guards the single-source rule: the
// number the endpoint shows and the number the seat guard rejects on come from
// the same bucketing. If someone ever adds a second way to count seats, the two
// drift and this fails.
func TestService_SeatsUsageMatchesEnforcement(t *testing.T) {
	active := [][]entity.Role{
		{entity.RoleAdmin, entity.RoleEditor},
		{entity.RoleReviewer},
		{entity.RoleEditor},
		{entity.RoleGuest},
		nil,
	}
	pending := [][]entity.Role{{entity.RoleEditor}, {entity.RoleGuest}}

	svc, m := newServiceWithMocks(t)
	m.store.EXPECT().Get(gomock.Any()).Return(licenseWithCap(10), nil)
	m.users.EXPECT().ListActiveRoles(gomock.Any()).Return(active, nil)
	m.invitations.EXPECT().ListPendingRoles(gomock.Any()).Return(pending, nil)

	usage, _, err := svc.SeatsUsage(context.Background())
	require.NoError(t, err)

	require.Equal(t, entity.BucketSeats(active, pending).SeatsOccupied(), usage.SeatsOccupied())
}

// TestService_SeatsUsageAcceptRaceDipsOccupied documents an accepted race, not a
// bug: collectSeatUsage reads active users and pending invitations in two
// separate statements under READ COMMITTED, and acceptance commits the
// pending→accepted flip and the role grant in one transaction. When that commit
// lands between the two reads, the invitee is already gone from pending and not
// yet visible in active, so the occupied total dips by one for that single
// response.
//
// The mocks model exactly that interleaving: the active read returns the
// pre-accept world, the pending read returns the post-accept one.
//
// Fixing it was considered and rejected — see the race decision section in
// tasks/plan.md: every remedy either introduces the codebase's first read-only
// isolation-level transaction, crosses module storage boundaries, or shifts the
// heartbeat's license report to Console. The window is microseconds and this is
// a polled read whose data is seconds stale by contract.
//
// What the assertion guards is the read ORDER: the gomock constraint below
// fails if the two reads in collectSeatUsage are ever swapped. Order is what
// fixes the direction of the skew — active-then-pending can only under-count
// across an accept, while the reverse would over-count. The reason we want
// under-counting here is argument rather than assertion: the same collection
// feeds the heartbeat's license report to Console, where claiming a seat too
// many is not harmless. No test covers that consequence; it lives in the
// heartbeat path.
func TestService_SeatsUsageAcceptRaceDipsOccupied(t *testing.T) {
	svc, m := newServiceWithMocks(t)
	m.store.EXPECT().Get(gomock.Any()).Return(licenseWithCap(10), nil)

	// Pre-accept snapshot: the invitee is not an active user yet.
	activeRead := m.users.EXPECT().
		ListActiveRoles(gomock.Any()).
		Return([][]entity.Role{{entity.RoleAdmin}}, nil)
	// Post-accept snapshot: the invitation is no longer pending. Reading it
	// strictly after the active half is what makes the miss possible at all.
	m.invitations.EXPECT().
		ListPendingRoles(gomock.Any()).
		Return(nil, nil).
		After(activeRead.Call)

	usage, _, err := svc.SeatsUsage(context.Background())
	require.NoError(t, err)

	require.EqualValues(t, 1, usage.SeatsOccupied(),
		"accepted race: the settled state is 2 occupied, one response reports 1")
}

// TestService_SeatsUsageTransitions covers the acceptance criterion that the
// counters track real events: sending an invite to a seat role raises pending,
// accepting moves that seat from pending to used, and revoke / expiry / block /
// downgrade give the seat back.
//
// It is written as deltas between two consecutive SeatsUsage calls on ONE
// service rather than as absolute numbers on a synthetic input. Absolute
// assertions on a single call only restate BucketSeats arithmetic; the delta is
// what the admin actually watches move, and it stays meaningful no matter which
// bucket the counting lives in.
//
// The two calls model the world before and after the event: the store mocks
// return the pre-event role sets first and the post-event ones second, which is
// exactly what the endpoint would read if it were polled either side of the
// event. The license is read once — it sits behind an hour-long TTL cache here —
// so only the role reads differ between the calls.
func TestService_SeatsUsageTransitions(t *testing.T) {
	// A settled world every case starts from or lands in: two active seat
	// holders and one active guest, no invitations outstanding.
	settledActive := [][]entity.Role{
		{entity.RoleAdmin}, {entity.RoleEditor}, {entity.RoleGuest},
	}
	// The same world with one seat invitation in flight.
	invitedPending := [][]entity.Role{{entity.RoleEditor}}
	// And the world after that invitation was accepted: the invitee is now an
	// active editor and nothing is pending.
	acceptedActive := [][]entity.Role{
		{entity.RoleAdmin}, {entity.RoleEditor}, {entity.RoleGuest}, {entity.RoleEditor},
	}

	type world struct {
		active  [][]entity.Role
		pending [][]entity.Role
	}
	type deltas struct{ used, pending, occupied int64 }

	tests := []struct {
		name          string
		before, after world
		want          deltas
	}{
		{
			// The invitation reserves a seat from the moment it is sent
			// (pending counts against the cap), so occupied climbs with it while
			// the heartbeat-reported used stays put — nobody has logged in yet.
			name:   "invite sent to a seat role reserves a seat",
			before: world{active: settledActive},
			after:  world{active: settledActive, pending: invitedPending},
			want:   deltas{used: 0, pending: +1, occupied: +1},
		},
		{
			// Acceptance is net-zero on occupied: the seat the invite was holding
			// is the same seat the new user now holds. This is the invariant that
			// keeps an admin at the cap from watching the number jump when an
			// invitee finally logs in.
			name:   "invite accepted moves the seat from pending to used",
			before: world{active: settledActive, pending: invitedPending},
			after:  world{active: acceptedActive},
			want:   deltas{used: +1, pending: -1, occupied: 0},
		},
		{
			// Revoke and expiry are the same event on the read path: the
			// invitation stops being pending while nobody became active. The
			// reserved seat comes back.
			name:   "pending invite revoked or expired releases the seat",
			before: world{active: settledActive, pending: invitedPending},
			after:  world{active: settledActive},
			want:   deltas{used: 0, pending: -1, occupied: -1},
		},
		{
			// A blocked user drops out of ListActiveRoles entirely.
			name:   "active seat user blocked releases the seat",
			before: world{active: settledActive},
			after:  world{active: [][]entity.Role{{entity.RoleAdmin}, {entity.RoleGuest}}},
			want:   deltas{used: -1, pending: 0, occupied: -1},
		},
		{
			// The user stays active, only the role changes — a guest is
			// read-only and holds no seat.
			name:   "seat role downgraded to guest releases the seat",
			before: world{active: settledActive},
			after: world{active: [][]entity.Role{
				{entity.RoleAdmin}, {entity.RoleGuest}, {entity.RoleGuest},
			}},
			want: deltas{used: -1, pending: 0, occupied: -1},
		},
		{
			// A guest invitation must not move a single counter.
			name:   "invite sent to guest moves nothing",
			before: world{active: settledActive},
			after:  world{active: settledActive, pending: [][]entity.Role{{entity.RoleGuest}}},
			want:   deltas{used: 0, pending: 0, occupied: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, m := newServiceWithMocks(t)
			// One license read for both calls: the hour-long TTL cache serves
			// the second. Only the role reads differ, which is the point.
			m.store.EXPECT().Get(gomock.Any()).Return(licenseWithCap(10), nil)

			beforeActive := m.users.EXPECT().ListActiveRoles(gomock.Any()).Return(tt.before.active, nil)
			m.users.EXPECT().ListActiveRoles(gomock.Any()).Return(tt.after.active, nil).After(beforeActive.Call)

			beforePending := m.invitations.EXPECT().ListPendingRoles(gomock.Any()).Return(tt.before.pending, nil)
			m.invitations.EXPECT().ListPendingRoles(gomock.Any()).Return(tt.after.pending, nil).After(beforePending.Call)

			before, _, err := svc.SeatsUsage(context.Background())
			require.NoError(t, err)

			after, _, err := svc.SeatsUsage(context.Background())
			require.NoError(t, err)

			require.Equal(t, tt.want.used, after.SeatsUsed()-before.SeatsUsed(),
				"seats_used moved from %d to %d", before.SeatsUsed(), after.SeatsUsed())
			require.Equal(t, tt.want.pending, after.SeatsPending()-before.SeatsPending(),
				"seats_pending moved from %d to %d", before.SeatsPending(), after.SeatsPending())
			require.Equal(t, tt.want.occupied, after.SeatsOccupied()-before.SeatsOccupied(),
				"seats_occupied moved from %d to %d", before.SeatsOccupied(), after.SeatsOccupied())
		})
	}
}

// TestService_SeatsUsageExactlyAtCap pins the boundary the widget exists to
// show: occupied has reached purchased, so the next invite is refused, and the
// read path still answers 200 with the plain numbers. The read must never
// borrow the guard's opinion — an admin sitting at the cap needs to see the
// numbers that explain the refusal, not an error.
//
// The mix is deliberate: three active seat holders and one pending invite make
// four occupied against a cap of four, so a bug that forgot the pending half
// would read as one seat still free at exactly the point where inviting fails.
func TestService_SeatsUsageExactlyAtCap(t *testing.T) {
	svc, m := newServiceWithMocks(t)
	m.store.EXPECT().Get(gomock.Any()).Return(licenseWithCap(4), nil)
	m.users.EXPECT().ListActiveRoles(gomock.Any()).Return([][]entity.Role{
		{entity.RoleAdmin}, {entity.RoleReviewer}, {entity.RoleEditor}, {entity.RoleGuest},
	}, nil)
	m.invitations.EXPECT().ListPendingRoles(gomock.Any()).Return([][]entity.Role{{entity.RoleEditor}}, nil)

	usage, lic, err := svc.SeatsUsage(context.Background())
	require.NoError(t, err, "being at the cap is a state to report, not an error")

	require.EqualValues(t, 3, usage.SeatsUsed())
	require.EqualValues(t, 1, usage.SeatsPending())
	require.NotNil(t, lic.SeatsPurchased)
	require.EqualValues(t, *lic.SeatsPurchased, usage.SeatsOccupied(),
		"the cap is reached exactly: the next seat invite is the one that gets refused")
}

// TestService_SeatsUsageZeroSeatCap covers a license that bought no seats at
// all. Zero is a real cap Console can report and must not be confused with the
// nil that means "no cap known": the report has to stay bounded rather than
// render as unlimited.
func TestService_SeatsUsageZeroSeatCap(t *testing.T) {
	svc, m := newServiceWithMocks(t)
	m.store.EXPECT().Get(gomock.Any()).Return(licenseWithCap(0), nil)
	m.users.EXPECT().ListActiveRoles(gomock.Any()).Return([][]entity.Role{{entity.RoleGuest}}, nil)
	m.invitations.EXPECT().ListPendingRoles(gomock.Any()).Return(nil, nil)

	usage, lic, err := svc.SeatsUsage(context.Background())
	require.NoError(t, err)

	require.Zero(t, usage.SeatsOccupied(), "a guest holds no seat even with no seats bought")
	require.NotNil(t, lic.SeatsPurchased, "zero seats bought is a cap, not an unknown one")
	require.Zero(t, *lic.SeatsPurchased)
}

func TestNoop_SeatsUsage(t *testing.T) {
	t.Parallel()

	usage, lic, err := NewNoop().SeatsUsage(context.Background())
	require.NoError(t, err)
	require.Nil(t, lic, "a nil license renders as unlimited")
	require.Equal(t, entity.SeatUsage{}, usage)
}

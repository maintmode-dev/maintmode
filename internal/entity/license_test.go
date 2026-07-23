package entity

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLicense_Blocked(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		license *License
		want    bool
	}{
		{name: "active is not blocked", license: &License{Status: LicenseStatusActive}},
		{name: "blocked status is blocked", license: &License{Status: LicenseStatusBlocked}, want: true},
		{name: "unknown status is not blocked", license: &License{Status: LicenseStatus("bogus")}},
		{name: "nil license is not blocked", license: nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tc.want, tc.license.Blocked())
		})
	}
}

func TestHighestRole(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		roles []Role
		want  Role
	}{
		{name: "admin wins over everything", roles: []Role{RoleGuest, RoleEditor, RoleReviewer, RoleAdmin}, want: RoleAdmin},
		{name: "reviewer wins over editor", roles: []Role{RoleEditor, RoleReviewer}, want: RoleReviewer},
		{name: "editor wins over guest", roles: []Role{RoleGuest, RoleEditor}, want: RoleEditor},
		{name: "guest only", roles: []Role{RoleGuest}, want: RoleGuest},
		{name: "empty set degrades to guest", roles: nil, want: RoleGuest},
		{name: "unknown role degrades to guest", roles: []Role{Role("bogus")}, want: RoleGuest},
		{name: "order does not matter", roles: []Role{RoleAdmin, RoleGuest}, want: RoleAdmin},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tc.want, HighestRole(tc.roles))
		})
	}
}

func TestBucketSeats(t *testing.T) {
	t.Parallel()

	active := [][]Role{
		{RoleAdmin, RoleEditor}, // multi-role user counts once, in admin
		{RoleReviewer},
		{RoleEditor},
		{RoleEditor},
		{RoleGuest},
		nil, // degraded row still occupies a seat
	}
	pending := [][]Role{
		{RoleAdmin},
		{RoleGuest},
		{RoleGuest},
	}

	usage := BucketSeats(active, pending)

	require.Equal(t, SeatBucket{Active: 1, Pending: 1}, usage.Admin)
	require.Equal(t, SeatBucket{Active: 1}, usage.Reviewer)
	require.Equal(t, SeatBucket{Active: 2}, usage.Editor)
	require.Equal(t, SeatBucket{Active: 2, Pending: 2}, usage.Guest)

	// The cap invariant: every active row lands in exactly one bucket.
	require.EqualValues(t, len(active), usage.TotalActive())
}

func TestSeatUsage_SeatsOccupied(t *testing.T) {
	t.Parallel()

	t.Run("counts active and pending across seat roles, excludes guest", func(t *testing.T) {
		t.Parallel()

		usage := SeatUsage{
			Admin:    SeatBucket{Active: 1, Pending: 1},
			Reviewer: SeatBucket{Active: 2, Pending: 0},
			Editor:   SeatBucket{Active: 1, Pending: 3},
			Guest:    SeatBucket{Active: 5, Pending: 4}, // never counted
		}

		require.EqualValues(t, 1+1+2+0+1+3, usage.SeatsOccupied())
	})

	t.Run("differs from SeatsUsed when pending invites exist", func(t *testing.T) {
		t.Parallel()

		// SeatsUsed (heartbeat) is Active-only; SeatsOccupied (guard) is
		// Active+Pending. A fixture with pending seat-invites must separate them,
		// or the guard would silently drop pending invites from the cap.
		usage := BucketSeats(
			[][]Role{{RoleAdmin}, {RoleEditor}},    // 2 active seats
			[][]Role{{RoleReviewer}, {RoleEditor}}, // 2 pending seats
		)

		require.EqualValues(t, 2, usage.SeatsUsed(), "Active-only")
		require.EqualValues(t, 4, usage.SeatsOccupied(), "Active+Pending")
		require.NotEqual(t, usage.SeatsUsed(), usage.SeatsOccupied())
	})

	t.Run("guest-only pending invites never occupy a seat", func(t *testing.T) {
		t.Parallel()

		usage := BucketSeats(nil, [][]Role{{RoleGuest}, {RoleGuest}})
		require.Zero(t, usage.SeatsOccupied())
	})
}

func TestRoleOccupiesSeat(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		role Role
		want bool
	}{
		{name: "admin occupies a seat", role: RoleAdmin, want: true},
		{name: "reviewer occupies a seat", role: RoleReviewer, want: true},
		{name: "editor occupies a seat", role: RoleEditor, want: true},
		{name: "guest does not occupy a seat", role: RoleGuest, want: false},
		{name: "unknown role does not occupy a seat", role: Role("bogus"), want: false},
		{name: "empty role does not occupy a seat", role: Role(""), want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tc.want, RoleOccupiesSeat(tc.role))
		})
	}
}

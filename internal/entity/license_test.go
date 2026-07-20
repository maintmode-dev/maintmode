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

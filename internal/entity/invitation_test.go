package entity

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// These tests pin the read-time behavior the invitation rotation relies on: a
// row the sweep has flipped to a persisted status='expired' must still resolve to
// "expired" through EffectiveStatus/PreviewStatus, exactly as a not-yet-rotated
// pending-past-expiry row does. Without these, the "persisting expired is safe
// for the read path" claim is unguarded.

func TestInvitation_EffectiveStatus(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	cases := []struct {
		name   string
		status InvitationStatus
		expiry time.Time
		want   InvitationStatus
	}{
		{"live pending stays pending", InvitationStatusPending, future, InvitationStatusPending},
		{"pending past expiry derives expired", InvitationStatusPending, past, InvitationStatusExpired},
		{"persisted expired reads expired even with future expiry", InvitationStatusExpired, future, InvitationStatusExpired},
		{"accepted is terminal regardless of expiry", InvitationStatusAccepted, past, InvitationStatusAccepted},
		{"revoked is terminal regardless of expiry", InvitationStatusRevoked, past, InvitationStatusRevoked},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			inv := &Invitation{Status: tc.status, ExpiresAt: tc.expiry}
			require.Equal(t, tc.want, inv.EffectiveStatus(now))
		})
	}
}

func TestInvitation_PreviewStatus(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	cases := []struct {
		name   string
		status InvitationStatus
		expiry time.Time
		want   InvitationPreviewStatus
	}{
		{"live pending is valid", InvitationStatusPending, future, InvitationPreviewStatusValid},
		{"pending past expiry is expired", InvitationStatusPending, past, InvitationPreviewStatusExpired},
		{"persisted expired is expired", InvitationStatusExpired, future, InvitationPreviewStatusExpired},
		{"accepted is accepted", InvitationStatusAccepted, past, InvitationPreviewStatusAccepted},
		{"revoked is revoked", InvitationStatusRevoked, past, InvitationPreviewStatusRevoked},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			inv := &Invitation{Status: tc.status, ExpiresAt: tc.expiry}
			require.Equal(t, tc.want, inv.PreviewStatus(now))
		})
	}
}

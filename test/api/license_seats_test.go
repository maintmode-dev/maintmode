//go:build api

package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

// TestAuthAPILicenseSeats covers the endpoint end to end: who may read it, and
// what the response looks like on this stack.
//
// The deployment used here configures no license, so enforcement is the noop
// implementation and the answer is always unlimited with zero counters — the
// counters moving with invites and role changes is therefore unobservable here
// by construction and is pinned at the service level instead. What this suite
// does prove is that the route exists, is reachable through the real
// middleware chain, and serializes the agreed shape.
func TestAuthAPILicenseSeats(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	t.Run("admin allowed", func(t *testing.T) {
		apiClient := setupAuthTestClientWithRoles(entity.RoleAdmin)

		resp, err := apiClient.GetApiV1LicenseSeatsWithResponse(ctx)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode(), "unexpected status: %s", resp.Body)
		require.NotNil(t, resp.JSON200)

		require.True(t, lo.FromPtr(resp.JSON200.Unlimited), "no license is configured on this stack")
		require.Zero(t, lo.FromPtr(resp.JSON200.SeatsUsed))
		require.Zero(t, lo.FromPtr(resp.JSON200.SeatsPending))
		require.Zero(t, lo.FromPtr(resp.JSON200.SeatsOccupied))

		// Asserted on the raw body: the generated client models every field as an
		// optional pointer, so a null ceiling and an omitted one decode alike, and
		// only the wire bytes show which one the server actually sent.
		require.JSONEq(t,
			`{"seats_purchased":null,"seats_used":0,"seats_pending":0,"seats_occupied":0,"unlimited":true}`,
			string(resp.Body),
		)
	})

	t.Run("editor forbidden", func(t *testing.T) {
		apiClient := setupAuthTestClientWithRoles(entity.RoleEditor)

		resp, err := apiClient.GetApiV1LicenseSeatsWithResponse(ctx)
		require.NoError(t, err)
		require.Equal(t, http.StatusForbidden, resp.StatusCode())
	})

	t.Run("reviewer forbidden", func(t *testing.T) {
		apiClient := setupAuthTestClientWithRoles(entity.RoleReviewer)

		resp, err := apiClient.GetApiV1LicenseSeatsWithResponse(ctx)
		require.NoError(t, err)
		require.Equal(t, http.StatusForbidden, resp.StatusCode())
	})

	t.Run("guest forbidden", func(t *testing.T) {
		apiClient := setupAuthTestClientWithRoles(entity.RoleGuest)

		resp, err := apiClient.GetApiV1LicenseSeatsWithResponse(ctx)
		require.NoError(t, err)
		require.Equal(t, http.StatusForbidden, resp.StatusCode())
	})

	t.Run("missing token unauthorized", func(t *testing.T) {
		apiClient := setupAuthTestClient()

		resp, err := apiClient.GetApiV1LicenseSeatsWithResponse(ctx)
		require.NoError(t, err)
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode())
	})
}

//go:build api

package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	authclient "github.com/ruko1202/maintmode/internal/pkg/generated/clients/auth"
	maintmodeclient "github.com/ruko1202/maintmode/internal/pkg/generated/clients/maintmode"
)

// TestMaintenancesAPI_ActiveTokenRevokedOnLogout is the end-to-end guard for the
// active-token middleware: a critical mutation (cancel) is gated by
// RequireActiveToken, which calls the real auth service's EnsureActiveToken
// (JWT + blacklist + blocked-user) on every request. After logout the access
// token is blacklisted, so the same token — still a valid, unexpired JWT — must
// be rejected with 401 on the gated mutation.
//
// The token must come from a REAL login (OAuth-stub exchange), not the locally
// minted synthetic JWTs the other suites use: logout blacklists the access
// token's JTI via its refresh session, and only an exchanged token has a JTI
// bound to a session that logout can revoke.
func TestMaintenancesAPI_ActiveTokenRevokedOnLogout(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	// Real login: a fresh stub user, auto-granted admin, gets a real access +
	// refresh pair whose access JTI is tied to a revocable session.
	authClient := newAuthTestClient("")
	exchangeResp, err := authClient.PostApiV1LoginOauthExchangeGoogleWithResponse(ctx,
		authclient.PostApiV1LoginOauthExchangeGoogleJSONRequestBody{
			IdToken: lo.ToPtr("api-test-active-token-" + uuid.NewString()),
		},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, exchangeResp.StatusCode(), "unexpected status: %s", exchangeResp.Body)
	require.NotNil(t, exchangeResp.JSON200)

	accessToken := lo.FromPtr(exchangeResp.JSON200.AccessToken)
	refreshToken := lo.FromPtr(exchangeResp.JSON200.RefreshToken)
	require.NotEmpty(t, accessToken)
	require.NotEmpty(t, refreshToken)

	// Client authenticated with the real session's access token.
	sessionClient := setupMaintmodeTestClientWithToken(accessToken)

	cancelReq := maintmodeclient.PostApiV1MaintenancesIdCancelJSONRequestBody{
		Reason:  lo.ToPtr(maintmodeclient.MaintenanceCancelReasonIncident),
		Comment: lo.ToPtr("active-token e2e"),
	}

	// Before logout: the real token passes the active-token gate, so the cancel
	// (a RequireActiveToken-gated mutation) succeeds.
	beforeID := createTestMaintenance(ctx, t, sessionClient)
	beforeResp, err := sessionClient.PostApiV1MaintenancesIdCancelWithResponse(ctx, uuid.MustParse(beforeID), cancelReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, beforeResp.StatusCode(),
		"cancel before logout should pass the active-token gate: %s", beforeResp.Body)

	// A second draft to cancel after logout — cancel is terminal, so we need a
	// fresh one. Created before logout while the token is still active.
	afterID := createTestMaintenance(ctx, t, sessionClient)

	// Log out: blacklists the access token's JTI.
	logoutResp, err := authClient.PostApiV1LogoutWithResponse(ctx,
		&authclient.PostApiV1LogoutParams{Authorization: lo.ToPtr("Bearer " + accessToken)},
		authclient.PostApiV1LogoutJSONRequestBody{RefreshToken: lo.ToPtr(refreshToken)},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, logoutResp.StatusCode(), "logout failed: %s", logoutResp.Body)

	// After logout: the same (still structurally valid) access token is now
	// blacklisted, so the active-token gate rejects the cancel with 401.
	afterResp, err := sessionClient.PostApiV1MaintenancesIdCancelWithResponse(ctx, uuid.MustParse(afterID), cancelReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, afterResp.StatusCode(),
		"cancel after logout must be rejected by the active-token gate: %s", afterResp.Body)
}

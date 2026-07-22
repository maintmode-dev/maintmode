//go:build api

package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	authclient "github.com/ruko1202/maintmode/internal/pkg/generated/clients/auth"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

// TestAuthAPI_AuthExchange_SignupDisabled pins the invite-only contract on the
// BFF exchange endpoint. The OAuth stub accepts any non-sentinel id_token and
// mints a fresh, unknown identity, so without an invitation, open signup
// (disabled in the test stack config), or the dev-only X-Test-Roles header the
// backend must refuse to provision the user: HTTP 403 with the stable machine
// code signup_disabled, no new users row, and a login.failed audit record with
// the dedicated "signup disabled" reason.
//
// The bootstrap fast path never fires here: TestMain seeded an active admin,
// so this login cannot win the first-admin bootstrap.
func TestAuthAPI_AuthExchange_SignupDisabled(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	adminClient := setupAuthTestClientWithRoles(entity.RoleAdmin)
	// Small backwards allowance so the audit filter tolerates clock skew
	// between the test host and the API container.
	auditSince := xtime.UTCNow().Add(-2 * time.Second)

	usersBefore := usersTotal(ctx, t, adminClient)

	resp, err := setupAuthTestClient().PostApiV1LoginOauthExchangeGoogleWithResponse(ctx,
		authclient.PostApiV1LoginOauthExchangeGoogleJSONRequestBody{
			IdToken: lo.ToPtr("api-test-signup-disabled"),
		})
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, resp.StatusCode(), "unexpected status: %s", resp.Body)
	require.NotNil(t, resp.JSON403, "expected the error envelope in the 403 body: %s", resp.Body)
	require.Equal(t, "signup_disabled", lo.FromPtr(resp.JSON403.Code))

	// The refused login must leave zero trace in the users table.
	require.Equal(t, usersBefore, usersTotal(ctx, t, adminClient),
		"refused signup must not create a user")

	// The refusal is audited as login.failed with the dedicated reason. Audit
	// records go through the durable outbox (eventually consistent), so poll
	// until the processor drains the entry. The poll error is carried out of
	// the closure so a persistent audit-endpoint failure is debuggable instead
	// of a bare timeout.
	var lastPollErr string
	found := assert.Eventually(t, func() bool {
		ok, pollErr := hasLoginFailedWithReason(ctx, adminClient, auditSince, string(entity.AuditFailureSignupDisabled))
		if pollErr != "" {
			lastPollErr = pollErr
		}
		return ok
	}, 30*time.Second, 500*time.Millisecond)
	require.Truef(t, found,
		"login.failed audit entry with reason %q not observed (last poll error: %s)",
		entity.AuditFailureSignupDisabled, lastPollErr)
}

// TestAuthAPI_AuthExchange_TestRolesGrantsRoles is the API-level smoke for the
// dev-only X-Test-Roles header; the parsing/branch matrix is covered by the
// handler and service unit tests. With the header, the exchange creates the
// stub user despite invite-only signup, and the minted access token carries the
// requested role on top of the default one.
func TestAuthAPI_AuthExchange_TestRolesGrantsRoles(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	resp, err := setupAuthTestClient().PostApiV1LoginOauthExchangeGoogleWithResponse(ctx,
		authclient.PostApiV1LoginOauthExchangeGoogleJSONRequestBody{
			IdToken: lo.ToPtr("api-test-test-roles"),
		},
		testRolesEditor(entity.RoleEditor),
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode(), "unexpected status: %s", resp.Body)
	require.NotNil(t, resp.JSON200)

	var claims entity.AccessClaims
	_, _, err = jwt.NewParser().ParseUnverified(lo.FromPtr(resp.JSON200.AccessToken), &claims)
	require.NoError(t, err)
	require.Contains(t, claims.UserRoles, entity.RoleEditor, "granted test role missing from access token")
	require.Contains(t, claims.UserRoles, entity.RoleGuest, "default role missing from access token")
}

// usersTotal reads the total number of users through the admin users list —
// the API-visible signal that a refused signup did not persist anything.
func usersTotal(ctx context.Context, t *testing.T, client *authclient.ClientWithResponses) int {
	t.Helper()

	resp, err := client.GetApiV1UsersListWithResponse(ctx, &authclient.GetApiV1UsersListParams{
		Limit: lo.ToPtr(1),
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode(), "unexpected status: %s", resp.Body)
	require.NotNil(t, resp.JSON200)

	return lo.FromPtr(resp.JSON200.Total)
}

// hasLoginFailedWithReason reports whether the audit log contains a
// login.failed entry created at or after since with the given failure reason.
// Transient failures are returned as a non-empty pollErr (and found=false) so
// the Eventually loop keeps polling but the last cause stays debuggable.
func hasLoginFailedWithReason(
	ctx context.Context,
	client *authclient.ClientWithResponses,
	since time.Time,
	reason string,
) (found bool, pollErr string) {
	resp, err := client.GetApiV1AuditLogWithResponse(ctx, &authclient.GetApiV1AuditLogParams{
		Limit:       lo.ToPtr(100),
		Action:      lo.ToPtr(authclient.LoginFailed),
		CreatedFrom: lo.ToPtr(since.Format(time.RFC3339)),
	})
	if err != nil {
		return false, err.Error()
	}
	if resp.StatusCode() != http.StatusOK || resp.JSON200 == nil {
		return false, fmt.Sprintf("audit log returned %d: %s", resp.StatusCode(), resp.Body)
	}

	for _, entry := range lo.FromPtr(resp.JSON200.Logs) {
		if lo.FromPtr(lo.FromPtr(entry.Metadata).FailureReason) == reason {
			return true, ""
		}
	}
	return false, ""
}

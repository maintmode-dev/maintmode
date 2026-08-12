//go:build api

package api

import (
	"context"
	"net/http"
	"net/mail"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	authclient "github.com/ruko1202/maintmode/internal/pkg/generated/clients/auth"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

func uniqueInviteEmail() string {
	return xuuid.NewString() + "@api-invite-test.com"
}

func strPtr(s string) *string { return &s }

func TestAuthAPIInvitations_Invite(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	// Note: the happy-path create/revoke/resend/accept flows are covered by the
	// service-level integration tests in internal/services/invitation, which can
	// seed a real inviting admin (the invited_by_id FK requires a persisted
	// user). The API-level tests here focus on routing, authorization, request
	// validation, and the public preview contract — all reachable with a
	// synthetic admin token.

	t.Run("editor forbidden", func(t *testing.T) {
		apiClient := setupAuthTestClientWithRoles(entity.RoleEditor)

		resp, err := apiClient.PostApiV1UsersInviteWithResponse(ctx, authclient.PostApiV1UsersInviteJSONRequestBody{
			Email: strPtr(uniqueInviteEmail()),
			Roles: &[]string{string(entity.RoleEditor)},
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusForbidden, resp.StatusCode())
	})

	t.Run("missing token unauthorized", func(t *testing.T) {
		apiClient := setupAuthTestClient()

		resp, err := apiClient.PostApiV1UsersInviteWithResponse(ctx, authclient.PostApiV1UsersInviteJSONRequestBody{
			Email: strPtr(uniqueInviteEmail()),
			Roles: &[]string{string(entity.RoleEditor)},
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode())
	})

	t.Run("empty roles rejected", func(t *testing.T) {
		apiClient := setupAuthTestClientWithRoles(entity.RoleAdmin)

		resp, err := apiClient.PostApiV1UsersInviteWithResponse(ctx, authclient.PostApiV1UsersInviteJSONRequestBody{
			Email: strPtr(uniqueInviteEmail()),
			Roles: &[]string{},
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), "unexpected status: %s", resp.Body)
	})

	t.Run("invalid email rejected", func(t *testing.T) {
		apiClient := setupAuthTestClientWithRoles(entity.RoleAdmin)

		resp, err := apiClient.PostApiV1UsersInviteWithResponse(ctx, authclient.PostApiV1UsersInviteJSONRequestBody{
			Email: strPtr("not-an-email"),
			Roles: &[]string{string(entity.RoleEditor)},
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), "unexpected status: %s", resp.Body)
	})
}

func TestAuthAPIInvitations_List(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	t.Run("admin allowed", func(t *testing.T) {
		apiClient := setupAuthTestClientWithRoles(entity.RoleAdmin)

		resp, err := apiClient.GetApiV1UsersInvitationsWithResponse(ctx, &authclient.GetApiV1UsersInvitationsParams{})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode(), "unexpected status: %s", resp.Body)
		require.NotNil(t, resp.JSON200)
	})

	t.Run("editor forbidden", func(t *testing.T) {
		apiClient := setupAuthTestClientWithRoles(entity.RoleEditor)

		resp, err := apiClient.GetApiV1UsersInvitationsWithResponse(ctx, &authclient.GetApiV1UsersInvitationsParams{})
		require.NoError(t, err)
		require.Equal(t, http.StatusForbidden, resp.StatusCode())
	})

	t.Run("unknown status filter rejected", func(t *testing.T) {
		apiClient := setupAuthTestClientWithRoles(entity.RoleAdmin)
		status := "bogus"

		resp, err := apiClient.GetApiV1UsersInvitationsWithResponse(ctx, &authclient.GetApiV1UsersInvitationsParams{Status: &status})
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode())
	})
}

func TestAuthAPIInvitations_Preview_Public(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	// Preview is public: no token required, and an unknown invitation token
	// returns 200 with status "invalid" — never 404, never leaking anything.
	apiClient := setupAuthTestClient()

	resp, err := apiClient.GetApiV1UsersInvitationsPreviewWithResponse(ctx, &authclient.GetApiV1UsersInvitationsPreviewParams{
		Token: "no-such-token",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode(), "unexpected status: %s", resp.Body)
	require.NotNil(t, resp.JSON200)
	require.Equal(t, "invalid", *resp.JSON200.Status)
	require.Nil(t, resp.JSON200.SuggestedProvider)
}

func TestAuthAPIInvitations_RevokeResend(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	t.Run("revoke unknown invitation gets not found", func(t *testing.T) {
		apiClient := setupAuthTestClientWithRoles(entity.RoleAdmin)

		resp, err := apiClient.PostApiV1UsersInvitationsIdRevokeWithResponse(ctx, uuid.New())
		require.NoError(t, err)
		require.Equal(t, http.StatusNotFound, resp.StatusCode(), "unexpected status: %s", resp.Body)
	})

	t.Run("revoke as editor forbidden", func(t *testing.T) {
		apiClient := setupAuthTestClientWithRoles(entity.RoleEditor)

		resp, err := apiClient.PostApiV1UsersInvitationsIdRevokeWithResponse(ctx, uuid.New())
		require.NoError(t, err)
		require.Equal(t, http.StatusForbidden, resp.StatusCode())
	})

	t.Run("resend unknown invitation gets not found", func(t *testing.T) {
		apiClient := setupAuthTestClientWithRoles(entity.RoleAdmin)

		resp, err := apiClient.PostApiV1UsersInvitationsIdResendWithResponse(ctx, uuid.New())
		require.NoError(t, err)
		require.Equal(t, http.StatusNotFound, resp.StatusCode(), "unexpected status: %s", resp.Body)
	})
}

// The stub OAuth provider accepts any token and mints an identity. This asserts
// the whole chain — request body → parser → service — is closed, not just its
// links in isolation. The API stack runs environment: dev, where the stub IS
// registered, so this covers the parser gate specifically: the refusal holds
// even where the provider exists. That the stub is also absent from the
// registry outside dev is a separate property, covered by the unit test in
// internal/services/oauthprovider.
func TestAuthAPIInvitations_AcceptRejectsStubProvider(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	apiClient := setupAuthTestClient()

	resp, err := apiClient.PostApiV1UsersInvitationsAcceptWithResponse(ctx,
		authclient.PostApiV1UsersInvitationsAcceptJSONRequestBody{
			InvitationToken: strPtr(xuuid.NewString()),
			OauthPayload: &authclient.ApimodelsOAuthPayload{
				Provider: strPtr("stub"),
				IdToken:  strPtr(uniqueInviteEmail()),
			},
		},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode(), "unexpected status: %s", resp.Body)
	require.NotNil(t, resp.JSON400)
	// "invalid" is the same opaque code an unknown token gets: the refusal must
	// not reveal that "stub" is a name the backend recognizes at all.
	require.Equal(t, "invalid", *resp.JSON400.Code)
}

// Canary for a failure mode chosen by the shape of the data, not by a flag: the
// stub returns a DETERMINISTIC identity for an id_token that parses as an email
// address, and a fresh random one otherwise. provisionUser and the TestMain
// seed both depend on the random branch — two calls must not collide on one
// user. An email-shaped token would make them silently share an identity and
// break the first-admin bootstrap.
//
// This scans the suite's source rather than listing tokens: a hand-copied list
// is a snapshot that goes stale the moment someone adds a login in another
// file, which is precisely the drift the canary exists to catch. If it fails,
// switch the stub to an explicit envelope rather than loosening the assertion.
func TestAuthAPIInvitations_SuiteTokensAreNotEmailShaped(t *testing.T) {
	sources, err := filepath.Glob("*_test.go")
	require.NoError(t, err)
	require.NotEmpty(t, sources)

	// Captures the string literal assigned to the id_token field through ANY
	// pointer helper, including the constant prefix of a concatenated form like
	// "api-test-" + xuuid.NewString(). Matching only one helper would miss a
	// login written with another one — this file itself uses strPtr while the
	// rest of the suite uses lo.ToPtr, so a single-helper pattern is already
	// blind to the form most likely to be copied next.
	idTokenLiteral := regexp.MustCompile(`IdToken:\s*[A-Za-z_.]+\("([^"]*)"`)

	var found int
	for _, src := range sources {
		body, err := os.ReadFile(src)
		require.NoError(t, err)

		for _, m := range idTokenLiteral.FindAllStringSubmatch(string(body), -1) {
			token := m[1]
			found++

			_, err := mail.ParseAddress(token)
			require.Error(t, err,
				"%s: id_token %q parses as an email, so the stub now mints a DETERMINISTIC identity for it; "+
					"logins that must yield distinct users would collide", src, token)
		}
	}

	// Guards the scan itself: a renamed field or client would silently match
	// nothing and leave this test green while checking absolutely nothing. The
	// floor is the number of logins the suite has today, so it also fails if a
	// rename takes out only some of them rather than all.
	require.GreaterOrEqual(t, found, 8, "id_token scan matched too little — has the client or field name changed?")
}

package userpicker

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5/echotest"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/userpicker/models"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
	testjsonudils "github.com/ruko1202/maintmode/test/utils/json"
)

// listAssignable calls the handler as the given caller, scoping the listing to
// one search term, and returns both the decoded response and the raw body. The
// raw body is what the security assertions run against: the struct cannot leak a
// field it does not have, so only the serialized wire format proves the point.
func listAssignable(
	ctx context.Context,
	t *testing.T,
	caller *entity.User,
	search string,
) (resp *apimodels.ListAssignableUsersResponse, rawBody string) {
	t.Helper()

	impl := initImpl(t)

	c, rec := echotest.ContextConfig{
		QueryValues: url.Values{"search": []string{search}},
	}.ToContextRecorder(t)
	c.SetRequest(c.Request().WithContext(ctx))
	xecho.UserToEchoCtx(c, caller)

	err := impl.ListAssignableUsers(c)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	body := rec.Body.String()
	decoded := testjsonudils.JSONToAny[apimodels.ListAssignableUsersResponse](t, rec.Body)

	return &decoded, body
}

func findUser(t *testing.T, resp *apimodels.ListAssignableUsersResponse, id uuid.UUID) *apimodels.AssignableUser {
	t.Helper()

	user, ok := lo.Find(resp.Users, func(u *apimodels.AssignableUser) bool { return u.ID == id })
	require.True(t, ok, "seeded user %s missing from picker response", id)

	return user
}

// TestListAssignableUsers_HasMessengerTag_EditorAndAbove asserts the flag is
// meaningful for every role permitted to plan a maintenance: it mirrors reality
// per row rather than being blanket true.
func TestListAssignableUsers_HasMessengerTag_EditorAndAbove(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// A shared, unique search term so the listing contains exactly the two
	// users this test seeded, regardless of what else lives in the database.
	marker := uuid.NewString()

	tagged := seedUserNamed(ctx, t, marker, "tagged", entity.DefaultRoles, lo.ToPtr("@tagged_handle"), nil)
	untagged := seedUserNamed(ctx, t, marker, "untagged", entity.DefaultRoles, nil, nil)

	for _, role := range []entity.Role{entity.RoleEditor, entity.RoleReviewer, entity.RoleAdmin} {
		t.Run(string(role), func(t *testing.T) {
			t.Parallel()

			resp, _ := listAssignable(ctx, t, callerWithRoles([]entity.Role{role}), marker)

			require.True(t, findUser(t, resp, tagged.ID).HasMessengerTag,
				"caller with role %s must see the tagged user's flag as true", role)
			require.False(t, findUser(t, resp, untagged.ID).HasMessengerTag,
				"caller with role %s must see the untagged user's flag as false", role)
		})
	}
}

// TestListAssignableUsers_HasMessengerTag_SlackOnly pins the aggregate: either
// handle alone sets the flag. It is not a telegram-only signal.
func TestListAssignableUsers_HasMessengerTag_SlackOnly(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	marker := uuid.NewString()

	slackOnly := seedUserNamed(ctx, t, marker, "slack", entity.DefaultRoles, nil, lo.ToPtr("U012SLACK"))

	resp, _ := listAssignable(ctx, t, callerWithRoles([]entity.Role{entity.RoleEditor}), marker)

	require.True(t, findUser(t, resp, slackOnly.ID).HasMessengerTag)
}

// TestListAssignableUsers_NeverLeaksTagValues is the negative security
// assertion, and it runs here rather than in the API e2e suite because
// `make tloc-api` does not run on pull requests — a check that only fires after
// merge is not a guard.
//
// It asserts on the raw marshaled body: neither tag key may appear, and no tag
// VALUE may appear as a substring anywhere in it, at any privilege level. A
// struct-level assertion would prove nothing, since a struct without the field
// cannot leak it; the wire format is the actual contract.
func TestListAssignableUsers_NeverLeaksTagValues(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	marker := uuid.NewString()

	// Values distinctive enough that an accidental appearance cannot be
	// coincidence, and that survive tag canonicalization unchanged.
	const (
		telegramTag = "leakcanary_telegram"
		slackTag    = "LEAKCANARYSLACK"
	)

	seedUserNamed(ctx, t, marker, "both", entity.DefaultRoles, lo.ToPtr(telegramTag), lo.ToPtr(slackTag))

	// Admin is the highest privilege level: if the values are absent here, they
	// are absent everywhere.
	for _, role := range []entity.Role{entity.RoleAdmin, entity.RoleEditor, entity.RoleGuest} {
		t.Run(string(role), func(t *testing.T) {
			t.Parallel()

			_, body := listAssignable(ctx, t, callerWithRoles([]entity.Role{role}), marker)

			require.NotContains(t, body, "telegram_tag",
				"the response must not carry a telegram_tag key")
			require.NotContains(t, body, "slack_tag",
				"the response must not carry a slack_tag key")

			lower := strings.ToLower(body)
			require.NotContains(t, lower, strings.ToLower(telegramTag),
				"the telegram tag VALUE leaked into the response body")
			require.NotContains(t, lower, strings.ToLower(slackTag),
				"the slack tag VALUE leaked into the response body")
		})
	}
}

package uiapprovals

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	approvalsmodels "github.com/ruko1202/maintmode/internal/app/api/ui/approvals/models"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
	testdbutils "github.com/ruko1202/maintmode/test/utils/db"
	testjsonudils "github.com/ruko1202/maintmode/test/utils/json"
)

// The queue is keyed on the caller's own id, so every subtest signs in as a
// freshly generated user. That yields a private slice of the shared DB and keeps
// total and ordering assertions stable under -count 2 and t.Parallel().
//
// Scope: what the handler itself owns — pagination coercion, the 401, the wire
// shape of the response, and that a row survives both mappers intact. Which rows
// come back at all (status filter, approver filter, ordering, total) is the
// store's query, tested exhaustively in list_pending_approvals_test.go; repeating
// a thinner version of it here would cost a DB round trip per case and fail in
// two places for one bug.
//
// There is no 403 case here on purpose: this test calls Implementation directly,
// with no middleware in the chain, so a role assertion at this level would pass
// no matter what the route is gated on. The role matrix lives in the e2e suite.
func TestListApprovals(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	impl := initImpl(t)

	t.Run("ok returns the caller's queue with pagination metadata", func(t *testing.T) {
		t.Parallel()
		approverID := xuuid.New()

		mine := makePendingMaint(ctx, t, approverID)
		makePendingMaint(ctx, t, xuuid.New()) // someone else's queue

		resp := listApprovals(t, impl, approverID, nil)

		require.EqualValues(t, 1, resp.Total)
		require.Len(t, resp.Maintenances, 1)
		require.Equal(t, mine.ID, resp.Maintenances[0].ID)
		require.EqualValues(t, 50, resp.Limit)
		require.EqualValues(t, 0, resp.Offset)
	})

	// Asserts the whole row, not just its id: every other subtest here compares
	// ids and pagination only, which leaves the entity -> calendardto -> ApprovalRow
	// mapping unguarded end to end. Dropping a single field from the service
	// mapper used to keep the whole suite green.
	t.Run("ok row carries every field through both mappers", func(t *testing.T) {
		t.Parallel()
		approverID, authorID := xuuid.New(), xuuid.New()

		maint := makePendingMaint(ctx, t, approverID,
			testdbutils.WithScope(entity.MaintenanceScopeGlobal),
			func(m *entity.Maintenance) { m.CreatedByUserID = authorID },
		)

		resp := listApprovals(t, impl, approverID, nil)

		require.Len(t, resp.Maintenances, 1)
		row := resp.Maintenances[0]

		require.Equal(t, maint.ID, row.ID)
		require.Equal(t, maint.Title, row.Title)
		require.Equal(t, string(entity.MaintenanceScopeGlobal), row.Scope)
		require.Equal(t, string(maint.Impact), row.Impact)
		require.True(t, maint.PlannedPeriod.Start.Equal(row.Start))
		require.True(t, lo.FromPtr(maint.PlannedPeriod.End).Equal(row.End))
		require.False(t, row.CreatedAt.IsZero())

		// created_by rides on CreatedByUserID: swap it for any other id in the
		// mapper and every author on the screen silently degrades to null.
		require.NotNil(t, row.CreatedBy)
		require.Equal(t, authorID, row.CreatedBy.ID)
		require.NotEmpty(t, row.CreatedBy.DisplayName)
	})

	t.Run("ok empty queue serializes as [] not null", func(t *testing.T) {
		t.Parallel()

		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
		c.SetRequest(c.Request().WithContext(ctx))
		signIn(c, xuuid.New())

		require.NoError(t, impl.ListApprovals(c))
		require.Equal(t, http.StatusOK, rec.Code)
		require.Contains(t, rec.Body.String(), `"maintenances":[]`,
			"an empty queue must not serialize as null — it would break .map() on the FE")
	})

	t.Run("no user in context is unauthorized", func(t *testing.T) {
		t.Parallel()

		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
		c.SetRequest(c.Request().WithContext(ctx))

		require.NoError(t, impl.ListApprovals(c))
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	// Malformed pagination is coerced, not rejected, and the response echoes the
	// coerced values so the FE paginates against what the server actually used.
	t.Run("invalid pagination falls back to defaults", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name       string
			query      url.Values
			wantLimit  int64
			wantOffset int64
		}{
			{name: "zero limit", query: url.Values{"limit": {"0"}}, wantLimit: 50},
			{name: "negative limit", query: url.Values{"limit": {"-1"}}, wantLimit: 50},
			{name: "non-numeric limit", query: url.Values{"limit": {"abc"}}, wantLimit: 50},
			{name: "limit above the cap falls back to 50, not to 200",
				query: url.Values{"limit": {"500"}}, wantLimit: 50},
			{name: "limit within the cap is honored",
				query: url.Values{"limit": {"7"}}, wantLimit: 7},
			// Both sides of the cap: the bound is exclusive (> max), so 200 is
			// still valid. Turning it into >= would slip past every other row here.
			{name: "limit exactly at the cap is honored",
				query: url.Values{"limit": {"200"}}, wantLimit: 200},
			{name: "one above the cap falls back",
				query: url.Values{"limit": {"201"}}, wantLimit: 50},
			{name: "negative offset", query: url.Values{"offset": {"-5"}}, wantLimit: 50},
			{name: "non-numeric offset", query: url.Values{"offset": {"abc"}}, wantLimit: 50},
			{name: "valid offset is honored",
				query: url.Values{"offset": {"3"}}, wantLimit: 50, wantOffset: 3},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				resp := listApprovals(t, impl, xuuid.New(), tt.query)

				require.Equal(t, tt.wantLimit, resp.Limit)
				require.Equal(t, tt.wantOffset, resp.Offset)
			})
		}
	})

	t.Run("ok limit and offset page the queue", func(t *testing.T) {
		t.Parallel()
		approverID := xuuid.New()

		makePendingMaint(ctx, t, approverID)
		second := makePendingMaint(ctx, t, approverID)

		page := listApprovals(t, impl, approverID, url.Values{"limit": {"1"}, "offset": {"1"}})

		require.EqualValues(t, 2, page.Total, "total must ignore limit/offset")
		// Exactly one row, and it is the second one — so offset skipped the first
		// rather than the page merely happening to contain it.
		require.Len(t, page.Maintenances, 1)
		require.Equal(t, second.ID, page.Maintenances[0].ID)
	})
}

func listApprovals(
	t *testing.T,
	impl *Implementation,
	approverID uuid.UUID,
	query url.Values,
) *approvalsmodels.ListApprovalsResponse {
	t.Helper()

	c, rec := echotest.ContextConfig{QueryValues: query}.ToContextRecorder(t)
	c.SetRequest(c.Request().WithContext(context.Background()))
	signIn(c, approverID)

	require.NoError(t, impl.ListApprovals(c))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	resp := testjsonudils.JSONToAny[approvalsmodels.ListApprovalsResponse](t, rec.Body)
	return &resp
}

// signIn puts the caller on the Echo context the way the auth middleware does in
// production — the approver of the queue is this user and nothing else.
func signIn(c *echo.Context, userID uuid.UUID) {
	xecho.UserToEchoCtx(c, &entity.User{
		ID:    userID,
		Email: "approver-" + userID.String() + "@example.com",
		Name:  "Approver " + userID.String(),
		Roles: entity.DefaultRoles,
	})
}

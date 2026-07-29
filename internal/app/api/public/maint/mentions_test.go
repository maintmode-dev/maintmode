package apimaint

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/maint/models"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
	testbootstraputils "github.com/ruko1202/maintmode/test/utils/bootstrap"
	testjsonudils "github.com/ruko1202/maintmode/test/utils/json"
)

// TestUpdateDraftMaintMentionsOmittedFieldPreservesThem is the gate-1 test.
// Service-level tests structurally cannot produce it: they build the command
// directly, so they can never observe an unconditional DTO->cmd mapping turning
// "field absent" into an empty slice — which every downstream gate then executes
// faithfully as "clear", wiping the mentions on every edit that does not resend
// them.
func TestUpdateDraftMaintMentionsOmittedFieldPreservesThem(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t)
	mentioned := []uuid.UUID{seedMentionableUser(ctx, t), seedMentionableUser(ctx, t)}

	draft := createDraftMaintenanceWithMentions(ctx, t, impl, mentioned)
	require.Len(t, draft.Mentions, 2, "create response must echo back what it persisted")

	// The edit omits mentions entirely — the shape a form sends when the user
	// touched only the title.
	updateDraftMaint(ctx, t, impl, draft.ID, func(req *apimodels.UpdateDraftMaintRequest) {
		req.Title = "Updated without touching mentions"
	})

	maint := getMaintByID(t, impl, draft.ID)
	require.Equal(t, mentioned, mentionUserIDsOf(maint.Mentions),
		"an edit that omits mentions must leave them exactly as they were")
}

// TestUpdateDraftMaintMentionsTriStateOverHTTP walks the remaining two states
// across the real handler, so the wire shape and the service gate are pinned
// together rather than only in isolation.
func TestUpdateDraftMaintMentionsTriStateOverHTTP(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t)

	t.Run("empty array clears mentions", func(t *testing.T) {
		t.Parallel()

		mentioned := []uuid.UUID{seedMentionableUser(ctx, t)}
		draft := createDraftMaintenanceWithMentions(ctx, t, impl, mentioned)

		updateDraftMaint(ctx, t, impl, draft.ID, func(req *apimodels.UpdateDraftMaintRequest) {
			req.Mentions = lo.ToPtr([]*apimodels.Mention{})
		})

		maint := getMaintByID(t, impl, draft.ID)
		require.Empty(t, maint.Mentions, "an empty array must clear the mentions")
	})

	t.Run("non-empty array replaces mentions", func(t *testing.T) {
		t.Parallel()

		mentioned := []uuid.UUID{seedMentionableUser(ctx, t)}
		draft := createDraftMaintenanceWithMentions(ctx, t, impl, mentioned)
		replacement := []uuid.UUID{seedMentionableUser(ctx, t), seedMentionableUser(ctx, t)}

		updateDraftMaint(ctx, t, impl, draft.ID, func(req *apimodels.UpdateDraftMaintRequest) {
			req.Mentions = lo.ToPtr(apimodels.ToAPIMentions(replacement))
		})

		maint := getMaintByID(t, impl, draft.ID)
		require.Equal(t, replacement, mentionUserIDsOf(maint.Mentions))
	})
}

// TestGetMaintMentionsCarryDisplayNames pins the read hydration plus the name
// resolution that rides along in the existing batch call.
func TestGetMaintMentionsCarryDisplayNames(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t)
	user := testbootstraputils.SeedEligibleApprover(ctx, t,
		testbootstraputils.InitServicesT(ctx, t, db, redis, cfg))

	draft := createDraftMaintenanceWithMentions(ctx, t, impl, []uuid.UUID{user.ID})

	maint := getMaintByID(t, impl, draft.ID)
	require.Len(t, maint.Mentions, 1)
	require.Equal(t, user.ID, maint.Mentions[0].UserID)
	require.Equal(t, user.Name, maint.Mentions[0].DisplayName)
	require.NotEqual(t, entity.UnknownUserName, maint.Mentions[0].DisplayName)
}

// TestUpdateDraftMaintRejectsOversizedMentions pins the API-level cap, which
// rejects before the request ever reaches the service.
func TestUpdateDraftMaintRejectsOversizedMentions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t)
	draft := createDraftMaintenance(ctx, t, impl)

	tooMany := make([]*apimodels.Mention, 0, maxMentions+1)
	for range maxMentions + 1 {
		tooMany = append(tooMany, &apimodels.Mention{UserID: uuid.New()})
	}

	req := validUpdateRequest(nil)
	req.Mentions = lo.ToPtr(tooMany)

	c, rec := echotest.ContextConfig{
		JSONBody: testjsonudils.AnyToJSONBytes(t, req),
	}.ToContextRecorder(t)
	c.SetPathValues(echo.PathValues{{Name: "id", Value: draft.ID.String()}})
	xecho.UserToEchoCtx(c, makeUser(t))

	require.NoError(t, impl.UpdateDraftMaint(c))
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}

// TestCreateDraftMaintRejectsOversizedMentions is the create-side twin.
func TestCreateDraftMaintRejectsOversizedMentions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t)

	tooMany := make([]*apimodels.Mention, 0, maxMentions+1)
	for range maxMentions + 1 {
		tooMany = append(tooMany, &apimodels.Mention{UserID: uuid.New()})
	}

	req := createDraftRequest(ctx, t)
	req.Mentions = tooMany

	c, rec := echotest.ContextConfig{
		JSONBody: testjsonudils.AnyToJSONBytes(t, req),
	}.ToContextRecorder(t)
	xecho.UserToEchoCtx(c, makeUser(t))

	require.NoError(t, impl.CreateDraftMaint(c))
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}

// TestToUpdateMaintenanceCmdMentionsTriState pins the only place that turns the
// wire pointer into the command's tri-state. The service decides "unchanged"
// versus "clear" purely by nil-ness of this field, so mapping unconditionally
// here would disable "unchanged" across the whole API while every service-level
// test stayed green — they build the command directly and never cross this
// mapping.
func TestToUpdateMaintenanceCmdMentionsTriState(t *testing.T) {
	t.Parallel()

	userID := uuid.New()

	tests := []struct {
		name      string
		requested *[]*apimodels.Mention
		assert    func(t *testing.T, got *[]*entity.MentionInput)
	}{
		{
			name:      "absent field leaves mentions unchanged",
			requested: nil,
			assert: func(t *testing.T, got *[]*entity.MentionInput) {
				t.Helper()
				require.Nil(t, got, "a missing field must stay nil so the service leaves mentions alone")
			},
		},
		{
			name:      "empty array clears mentions",
			requested: lo.ToPtr([]*apimodels.Mention{}),
			assert: func(t *testing.T, got *[]*entity.MentionInput) {
				t.Helper()
				require.NotNil(t, got, "an empty array must survive as a non-nil pointer, or clearing turns into a no-op")
				require.Empty(t, lo.FromPtr(got))
			},
		},
		{
			name:      "non-empty array replaces mentions",
			requested: lo.ToPtr([]*apimodels.Mention{{UserID: userID}}),
			assert: func(t *testing.T, got *[]*entity.MentionInput) {
				t.Helper()
				require.NotNil(t, got)
				require.Len(t, lo.FromPtr(got), 1)
				require.Equal(t, userID, lo.FromPtr(got)[0].UserID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := validUpdateRequest(nil)
			req.Mentions = tt.requested

			cmd, err := toUpdateMaintenanceCmd(context.Background(), uuid.New(), req)
			require.NoError(t, err)

			tt.assert(t, cmd.Mentions)
		})
	}
}

// seedMentionableUser provisions a real, persisted, unblocked user and returns
// its id. The mention-eligibility check runs against the real user backend, so a
// random uuid would be rejected.
func seedMentionableUser(ctx context.Context, t *testing.T) uuid.UUID {
	t.Helper()

	services := testbootstraputils.InitServicesT(ctx, t, db, redis, cfg)

	return testbootstraputils.SeedEligibleApprover(ctx, t, services).ID
}

func mentionUserIDsOf(views []*apimodels.MentionView) []uuid.UUID {
	return lo.Map(views, func(v *apimodels.MentionView, _ int) uuid.UUID {
		return v.UserID
	})
}

// createDraftRequest builds an otherwise-valid create payload so a test can vary
// only the field under assertion.
func createDraftRequest(ctx context.Context, t *testing.T) *apimodels.CreateDraftMaintRequest {
	t.Helper()

	notifyChan := makeNotifyChannel(ctx, t)
	resource := createResource(ctx, t)
	plannedStart := time.Now().
		AddDate(100, 0, 0).
		Add(time.Duration(testMaintenanceIndex.Add(1)) * 2 * time.Hour)

	return &apimodels.CreateDraftMaintRequest{
		Title:        "Test maintenance " + uuid.New().String()[:8],
		Description:  "Test description",
		PlannedStart: plannedStart,
		Scope:        apimodels.MaintenanceScopeResources,
		Impact:       apimodels.MaintenanceImpactPartial,
		Resources:    []*apimodels.ResourceRef{resource},
		Steps: []*apimodels.MaintenanceStepInput{{
			Order:               1,
			Description:         "Step 1",
			RollbackDescription: "Rollback step 1",
			Duration:            "30m",
		}},
		NotifyTargets:  &apimodels.NotifyTargets{ChannelIDs: []string{notifyChan.ID}},
		ApproverUserID: seedApprover(ctx, t),
	}
}

func createDraftMaintenanceWithMentions(
	ctx context.Context,
	t *testing.T,
	impl *Implementation,
	mentioned []uuid.UUID,
) *apimodels.CreateDraftMaintResponse {
	t.Helper()

	req := createDraftRequest(ctx, t)
	req.Mentions = apimodels.ToAPIMentions(mentioned)

	c, rec := echotest.ContextConfig{
		JSONBody: testjsonudils.AnyToJSONBytes(t, req),
	}.ToContextRecorder(t)
	xecho.UserToEchoCtx(c, makeUser(t))

	require.NoError(t, impl.CreateDraftMaint(c))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	resp := testjsonudils.JSONToAny[apimodels.CreateDraftMaintResponse](t, rec.Body)

	return &resp
}

// updateDraftMaint sends an otherwise-valid edit, letting the caller mutate the
// request first. The base payload deliberately leaves Mentions unset, so a test
// that does not touch it exercises the "field absent" path.
func updateDraftMaint(
	ctx context.Context,
	t *testing.T,
	impl *Implementation,
	maintID uuid.UUID,
	mutate func(req *apimodels.UpdateDraftMaintRequest),
) {
	t.Helper()

	// validUpdateRequest carries a placeholder channel id, which is enough for
	// the pure-validation tests but is rejected by the service; a real edit needs
	// a channel that exists in the catalog.
	req := validUpdateRequest(nil)
	req.NotifyTargets = &apimodels.NotifyTargets{ChannelIDs: []string{makeNotifyChannel(ctx, t).ID}}
	mutate(req)

	c, rec := echotest.ContextConfig{
		JSONBody: testjsonudils.AnyToJSONBytes(t, req),
	}.ToContextRecorder(t)
	c.SetPathValues(echo.PathValues{{Name: "id", Value: maintID.String()}})
	xecho.UserToEchoCtx(c, makeUser(t))

	require.NoError(t, impl.UpdateDraftMaint(c))
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
}

package maint

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// TestValidateApproverQueryShape pins the filter shape the eligibility check
// sends to the user service. A regression to Limit:0 (LIMIT 0 → no rows) or a
// dropped ExcludeBlocked/Roles filter would silently reject every approver or
// admit a blocked/non-reviewer one, while the behavior-only tests stayed green.
func TestValidateApproverQueryShape(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	approverID := uuid.New()

	service, mocks := initService(t)

	var gotCmd *entity.ListUsersCmd
	mocks.users.EXPECT().
		ListUsers(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, cmd *entity.ListUsersCmd) (*entity.ListUsersResult, error) {
			gotCmd = cmd
			return &entity.ListUsersResult{
				Users: []*entity.User{{ID: approverID, Roles: entity.ApproverEligibleRoles}},
				Total: 1,
			}, nil
		}).
		AnyTimes()

	_, err := service.CreateDraft(ctx, validCreateCmd(ctx, t, service, approverID))
	require.NoError(t, err)

	require.Equal(t, []uuid.UUID{approverID}, gotCmd.IDs)
	require.Equal(t, entity.ApproverEligibleRoles, gotCmd.Roles)
	require.True(t, gotCmd.ExcludeBlocked, "approver check must exclude blocked users")
	require.Positive(t, gotCmd.Limit, "Limit must be > 0 or the store emits LIMIT 0 and returns no rows")
}

func TestCreateDraftValidatesApprover(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("rejects ineligible approver", func(t *testing.T) {
		t.Parallel()
		approverID := uuid.New()

		service, mocks := initService(t)
		// approverID is not in the eligible set → empty ListUsers result.
		mocks.expectEligibleApprover()

		_, err := service.CreateDraft(ctx, validCreateCmd(ctx, t, service, approverID))
		require.ErrorIs(t, err, apperr.ErrApproverNotEligible)
		require.ErrorIs(t, err, apperr.ErrValidation)
	})

	t.Run("propagates auth outage", func(t *testing.T) {
		t.Parallel()

		approverID := uuid.New()
		service, mocks := initService(t)
		mocks.users.EXPECT().
			ListUsers(gomock.Any(), gomock.Any()).
			Return(nil, apperr.ErrAuthUnavailable)

		_, err := service.CreateDraft(ctx, validCreateCmd(ctx, t, service, approverID))
		require.ErrorIs(t, err, apperr.ErrAuthUnavailable)
	})

	t.Run("accepts eligible approver", func(t *testing.T) {
		t.Parallel()

		service, mocks := initService(t)
		approverID := uuid.New()
		mocks.expectEligibleApprover(approverID)

		cmd := validCreateCmd(ctx, t, service, approverID)
		maint, err := service.CreateDraft(ctx, cmd)
		require.NoError(t, err)
		require.NotNil(t, maint)
		require.Equal(t, cmd.ApproverUserID, maint.ApproverUserID)
	})
}

func TestUpdateMaintValidatesApprover(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("rejects ineligible new approver", func(t *testing.T) {
		t.Parallel()

		oldApprover := uuid.New()
		newApprover := uuid.New()

		service, mocks := initService(t)
		// oldApprover is eligible; newApprover is not (absent from the set).
		mocks.expectEligibleApprover(oldApprover)

		// Create a draft with an eligible approver, then try to reassign it to
		// an ineligible one.
		created, err := service.CreateDraft(ctx, validCreateCmd(ctx, t, service, oldApprover))
		require.NoError(t, err)

		err = service.UpdateMaint(ctx, &entity.UpdateMaintenanceCmd{
			MaintID:        created.ID,
			ApproverUserID: &newApprover,
		})
		require.ErrorIs(t, err, apperr.ErrApproverNotEligible)
	})

	t.Run("skips validation when approver unchanged", func(t *testing.T) {
		t.Parallel()

		// The original approver is registered as ineligible; since the update
		// leaves the approver unchanged (nil), the validator must not reject it.
		approverID := uuid.New()

		service, mocks := initService(t)
		mocks.expectEligibleApprover(approverID)

		created, err := service.CreateDraft(ctx, validCreateCmd(ctx, t, service, approverID))
		require.NoError(t, err)

		newTitle := "Updated" + t.Name()
		err = service.UpdateMaint(ctx, &entity.UpdateMaintenanceCmd{
			MaintID: created.ID,
			Title:   &newTitle,
		})
		require.NoError(t, err)
	})
}

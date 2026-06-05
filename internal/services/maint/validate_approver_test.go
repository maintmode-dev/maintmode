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

func TestCreateDraftValidatesApprover(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("rejects ineligible approver", func(t *testing.T) {
		t.Parallel()
		approverID := uuid.New()

		service, mocks := initService(t)
		mocks.approverValidator.EXPECT().
			IsEligibleApprover(gomock.Any(), approverID).
			Return(false, nil)

		_, err := service.CreateDraft(ctx, validCreateCmd(ctx, t, service, approverID))
		require.ErrorIs(t, err, apperr.ErrApproverNotEligible)
		require.ErrorIs(t, err, apperr.ErrValidation)
	})

	t.Run("propagates auth outage", func(t *testing.T) {
		t.Parallel()

		approverID := uuid.New()
		service, mocks := initService(t)
		mocks.approverValidator.EXPECT().
			IsEligibleApprover(gomock.Any(), approverID).
			Return(false, apperr.ErrAuthUnavailable)

		_, err := service.CreateDraft(ctx, validCreateCmd(ctx, t, service, approverID))
		require.ErrorIs(t, err, apperr.ErrAuthUnavailable)
	})

	t.Run("accepts eligible approver", func(t *testing.T) {
		t.Parallel()

		service, mocks := initService(t)
		mocks.approverValidator.EXPECT().
			IsEligibleApprover(gomock.Any(), gomock.Any()).
			Return(true, nil).
			AnyTimes()

		// Default stub treats any unregistered id as eligible.
		cmd := validCreateCmd(ctx, t, service, uuid.New())
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
		mocks.approverValidator.EXPECT().
			IsEligibleApprover(gomock.Any(), oldApprover).
			Return(true, nil)
		mocks.approverValidator.EXPECT().
			IsEligibleApprover(gomock.Any(), newApprover).
			Return(false, nil)

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
		mocks.approverValidator.EXPECT().
			IsEligibleApprover(gomock.Any(), approverID).
			Return(true, nil)

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

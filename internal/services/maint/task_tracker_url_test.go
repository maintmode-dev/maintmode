package maint

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	testdbutils "github.com/ruko1202/maintmode/test/utils/db"

	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

func TestCreateDraftStoresTaskTrackerURL(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	service, mocks := initService(t)
	mocks.expectAnyApproverEligible()

	cmd := validCreateCmd(ctx, t, service, uuid.New())
	cmd.TaskTrackerURL = "https://jira.example.com/browse/OPS-" + uuid.NewString()

	created, err := service.CreateDraft(ctx, cmd)
	require.NoError(t, err)
	require.Equal(t, cmd.TaskTrackerURL, created.TaskTrackerURL)

	stored, err := service.maintStore.GetMaint(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, cmd.TaskTrackerURL, stored.TaskTrackerURL)
}

func TestUpdateMaintTaskTrackerURL(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := xtime.UTCNow().Add(uniqueFutureOffset())

	// draftWithLink seeds a draft and sets its link through the update path, so
	// every subtest starts from a stored link. Each subtest's own update is the
	// last MaintUpdated it records, which is what lastAction reads.
	draftWithLink := func(t *testing.T, service *Service) (*entity.Maintenance, string) {
		t.Helper()

		maint := testdbutils.MakeMaint(ctx, t, service.maintStore, service.resourcesStore,
			entity.NewPeriod(now, now.Add(time.Hour)),
			testdbutils.WithScope(entity.MaintenanceScopeResources),
		)
		link := "https://jira.example.com/browse/OPS-" + uuid.NewString()
		require.NoError(t, service.UpdateMaint(ctx, &entity.UpdateMaintenanceCmd{
			MaintID:        maint.ID,
			TaskTrackerURL: lo.ToPtr(link),
			Actor:          actor(),
		}))

		return maint, link
	}

	storedURL := func(t *testing.T, service *Service, maintID uuid.UUID) string {
		t.Helper()
		stored, err := service.maintStore.GetMaint(ctx, maintID)
		require.NoError(t, err)
		return stored.TaskTrackerURL
	}

	t.Run("set records an audit change", func(t *testing.T) {
		t.Parallel()
		service, mocks := initService(t)
		maint, link := draftWithLink(t, service)

		next := link + "-next"
		require.NoError(t, service.UpdateMaint(ctx, &entity.UpdateMaintenanceCmd{
			MaintID:        maint.ID,
			TaskTrackerURL: lo.ToPtr(next),
			Actor:          actor(),
		}))

		require.Equal(t, next, storedURL(t, service, maint.ID))
		got := lastAction[audit.MaintUpdated](t, mocks.audit)
		require.Contains(t, got.Changes, entity.AuditFieldChange{Field: "task_tracker_url", Old: link, New: next})
	})

	t.Run("empty string clears the link", func(t *testing.T) {
		t.Parallel()
		service, mocks := initService(t)
		maint, link := draftWithLink(t, service)

		require.NoError(t, service.UpdateMaint(ctx, &entity.UpdateMaintenanceCmd{
			MaintID:        maint.ID,
			TaskTrackerURL: lo.ToPtr(""),
			Actor:          actor(),
		}))

		require.Empty(t, storedURL(t, service, maint.ID))
		got := lastAction[audit.MaintUpdated](t, mocks.audit)
		require.Contains(t, got.Changes, entity.AuditFieldChange{Field: "task_tracker_url", Old: link, New: ""})
	})

	t.Run("absent field leaves the link alone", func(t *testing.T) {
		t.Parallel()
		service, mocks := initService(t)
		maint, link := draftWithLink(t, service)

		require.NoError(t, service.UpdateMaint(ctx, &entity.UpdateMaintenanceCmd{
			MaintID: maint.ID,
			Title:   lo.ToPtr("renamed " + t.Name()),
			Actor:   actor(),
		}))

		require.Equal(t, link, storedURL(t, service, maint.ID))
		got := lastAction[audit.MaintUpdated](t, mocks.audit)
		for _, change := range got.Changes {
			require.NotEqual(t, "task_tracker_url", change.Field)
		}
	})

	t.Run("invalid link is refused", func(t *testing.T) {
		t.Parallel()
		service, _ := initService(t)
		maint, link := draftWithLink(t, service)

		require.Error(t, service.UpdateMaint(ctx, &entity.UpdateMaintenanceCmd{
			MaintID:        maint.ID,
			TaskTrackerURL: lo.ToPtr("ftp://jira.example.com/OPS-1"),
			Actor:          actor(),
		}))
		require.Equal(t, link, storedURL(t, service, maint.ID))
	})

	// Every lifecycle transition rewrites the whole row; the link must come out
	// of approve and start intact.
	t.Run("link survives approve and start", func(t *testing.T) {
		t.Parallel()
		service, _ := initService(t)
		maint, link := draftWithLink(t, service)

		draft, err := service.maintStore.GetMaint(ctx, maint.ID)
		require.NoError(t, err)
		conflicts, err := service.conflictsSrv.GetConflicts(ctx, &entity.ConflictQueryCmd{
			MaintID:       draft.ID,
			PlannedPeriod: draft.PlannedPeriod,
			Scope:         draft.Scope,
			ResourceIDs:   draft.Resources,
		})
		require.NoError(t, err)

		approver := actor()
		approver.ID = draft.ApproverUserID
		require.NoError(t, service.ApproveMaint(ctx, &entity.ApproveMaintenanceCmd{
			MaintID:               draft.ID,
			ObservedMaintRevision: draft.Revision(),
			ActorUserID:           draft.ApproverUserID,
			Actor:                 approver,
			ConflictSnapshot:      entity.ConflictsSnapshot{Conflicts: conflicts},
		}))
		require.Equal(t, link, storedURL(t, service, maint.ID))

		require.NoError(t, service.StartMaint(ctx, &entity.StartMaintenanceCmd{MaintID: maint.ID, Actor: actor()}))
		require.Equal(t, link, storedURL(t, service, maint.ID))
	})
}

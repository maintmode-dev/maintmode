package test

import (
	"context"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/utils/xtime"

	"github.com/ruko1202/maintmode/internal/apperr"

	"github.com/ruko1202/maintmode/internal/entity"
	testbootstraputils "github.com/ruko1202/maintmode/test/utils/bootstrap"
	testdbutils "github.com/ruko1202/maintmode/test/utils/db"
)

func TestApprove(t *testing.T) {
	ctx := context.Background()

	services := testbootstraputils.InitServicesT(ctx, t, db, valkey, cfg)
	s := services.Maint
	conflictsSrv := services.Conflicts

	// A real, persisted, approver-eligible user. Its id is threaded into the
	// maintenances below as the assigned approver, so approve calls acting as that
	// user pass the assigned-approver guard against the real user backend.
	approver := testbootstraputils.SeedEligibleApprover(ctx, t, services)

	t.Run("ok", func(t *testing.T) {
		start, end := testdbutils.IsolatedPeriodBounds(t)

		sharedResource := testdbutils.MakeResource(ctx, t, resourcesStore)

		conflictedMaints := []*entity.Maintenance{
			// using sharedResource
			testdbutils.MakeMaint(ctx, t, maintStore, resourcesStore,
				entity.NewPeriod(start.Add(time.Hour), end.Add(-time.Hour)),
				testdbutils.WithStatus(entity.MaintenanceStatusPlanned),
				testdbutils.WithScope(entity.MaintenanceScopeResources),
				testdbutils.WithResources(
					sharedResource.ID,
					testdbutils.MakeResource(ctx, t, resourcesStore).ID,
				),
			),
			// global scope
			testdbutils.MakeMaint(ctx, t, maintStore, resourcesStore,
				entity.NewPeriod(start, end),
				testdbutils.WithStatus(entity.MaintenanceStatusPlanned),
				testdbutils.WithScope(entity.MaintenanceScopeGlobal),
			),
		}
		for _, m := range conflictedMaints {
			err := s.StartMaint(ctx, &entity.StartMaintenanceCmd{MaintID: m.ID, Actor: testActor()})
			require.NoError(t, err)
		}

		maint := testdbutils.MakeMaint(ctx, t, maintStore, resourcesStore,
			entity.NewPeriod(start, end),
			testdbutils.WithScope(entity.MaintenanceScopeResources),
			testdbutils.WithApprover(approver.ID),
			testdbutils.WithResources(
				sharedResource.ID,
				testdbutils.MakeResource(ctx, t, resourcesStore).ID,
			),
		)

		actualConflicts, err := conflictsSrv.GetConflicts(ctx, &entity.ConflictQueryCmd{
			MaintID:       maint.ID,
			PlannedPeriod: maint.PlannedPeriod,
			Scope:         maint.Scope,
			ResourceIDs:   maint.Resources,
		})
		require.NoError(t, err)

		err = s.ApproveMaint(ctx, &entity.ApproveMaintenanceCmd{
			MaintID:               maint.ID,
			ObservedMaintRevision: maint.Revision(),
			ActorUserID:           maint.ApproverUserID,
			Actor:                 &entity.User{ID: maint.ApproverUserID, Email: "approver@example.com", Name: "Approver"},
			ConflictSnapshot: entity.ConflictsSnapshot{
				Conflicts: actualConflicts,
			},
		})
		require.NoError(t, err)

		actualMaint, err := s.GetMaint(ctx, maint.ID)
		require.NoError(t, err)
		require.Equal(t, entity.MaintenanceStatusPlanned, actualMaint.Status)
	})

	t.Run("ErrApproverMismatch", func(t *testing.T) {
		start, end := testdbutils.IsolatedPeriodBounds(t)

		maint := testdbutils.MakeMaint(ctx, t, maintStore, resourcesStore,
			entity.NewPeriod(start, end),
			testdbutils.WithScope(entity.MaintenanceScopeResources),
		)

		actualConflicts, err := conflictsSrv.GetConflicts(ctx, &entity.ConflictQueryCmd{
			MaintID:       maint.ID,
			PlannedPeriod: maint.PlannedPeriod,
			Scope:         maint.Scope,
			ResourceIDs:   maint.Resources,
		})
		require.NoError(t, err)

		// A non-admin user other than the assigned approver may not approve.
		other := testActor()
		err = s.ApproveMaint(ctx, &entity.ApproveMaintenanceCmd{
			MaintID:               maint.ID,
			ObservedMaintRevision: maint.Revision(),
			ActorUserID:           other.ID,
			Actor:                 other,
			ConflictSnapshot: entity.ConflictsSnapshot{
				Conflicts: actualConflicts,
			},
		})
		require.ErrorIs(t, err, apperr.ErrApproverMismatch)
	})

	// Who, other than the assigned approver, may approve a draft. Only an active
	// admin gets the override; RBAC has already let all of these through, so the
	// assignment guard alone decides.
	blockedAdmin := testActor(entity.RoleAdmin)
	blockedAdmin.BlockedAt = lo.ToPtr(xtime.UTCNow())

	overrideTests := []struct {
		name  string
		actor *entity.User
		// wantErr is nil when the override applies and the maintenance is expected
		// to reach "planned".
		wantErr error
	}{
		{
			// An admin unblocks a maintenance whose assigned approver cannot act
			// (left the company, was demoted or blocked).
			name:  "admin approves a maintenance assigned to someone else",
			actor: testActor(entity.RoleAdmin),
		},
		{
			// A blocked admin keeps the role but has lost access. In production the
			// RequireActiveToken middleware rejects them before the service runs,
			// and an actor built from a token never carries BlockedAt — so this
			// case pins the predicate, not a reachable path: it fails if the guard
			// is ever relaxed back to IsAdmin.
			name:    "blocked admin does not get the override",
			actor:   blockedAdmin,
			wantErr: apperr.ErrApproverMismatch,
		},
		{
			// A reviewer holds maintenance.approve, so RBAC lets the request
			// through; only the assignment guard stops them.
			name:    "non-admin approver-eligible role stays bound to the assignment",
			actor:   testActor(entity.RoleReviewer),
			wantErr: apperr.ErrApproverMismatch,
		},
	}
	for _, tt := range overrideTests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := testdbutils.IsolatedPeriodBounds(t)

			maint := testdbutils.MakeMaint(ctx, t, maintStore, resourcesStore,
				entity.NewPeriod(start, end),
				testdbutils.WithScope(entity.MaintenanceScopeResources),
				testdbutils.WithApprover(approver.ID),
			)

			actualConflicts, err := conflictsSrv.GetConflicts(ctx, &entity.ConflictQueryCmd{
				MaintID:       maint.ID,
				PlannedPeriod: maint.PlannedPeriod,
				Scope:         maint.Scope,
				ResourceIDs:   maint.Resources,
			})
			require.NoError(t, err)

			err = s.ApproveMaint(ctx, &entity.ApproveMaintenanceCmd{
				MaintID:               maint.ID,
				ObservedMaintRevision: maint.Revision(),
				ActorUserID:           tt.actor.ID,
				Actor:                 tt.actor,
				ConflictSnapshot: entity.ConflictsSnapshot{
					Conflicts: actualConflicts,
				},
			})
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)

			actualMaint, err := s.GetMaint(ctx, maint.ID)
			require.NoError(t, err)
			require.Equal(t, entity.MaintenanceStatusPlanned, actualMaint.Status)
		})
	}

	t.Run("ErrNilActor", func(t *testing.T) {
		start, end := testdbutils.IsolatedPeriodBounds(t)

		maint := testdbutils.MakeMaint(ctx, t, maintStore, resourcesStore,
			entity.NewPeriod(start, end),
			testdbutils.WithScope(entity.MaintenanceScopeResources),
			testdbutils.WithApprover(approver.ID),
		)

		// The approve decision reads the actor's roles, so a missing actor must
		// fail loudly instead of silently falling back to the strict assignment
		// check.
		err := s.ApproveMaint(ctx, &entity.ApproveMaintenanceCmd{
			MaintID:               maint.ID,
			ObservedMaintRevision: maint.Revision(),
			ActorUserID:           maint.ApproverUserID,
		})
		require.ErrorIs(t, err, apperr.ErrNilActor)
	})

	t.Run("ErrForbiddenStatusTransition", func(t *testing.T) {
		start, end := testdbutils.IsolatedPeriodBounds(t)

		maint := testdbutils.MakeMaint(ctx, t, maintStore, resourcesStore,
			entity.NewPeriod(start, end),
			testdbutils.WithScope(entity.MaintenanceScopeResources),
			testdbutils.WithStatus(entity.MaintenanceStatusPlanned),
		)

		actualConflicts, err := conflictsSrv.GetConflicts(ctx, &entity.ConflictQueryCmd{
			MaintID:       maint.ID,
			PlannedPeriod: maint.PlannedPeriod,
			Scope:         maint.Scope,
			ResourceIDs:   maint.Resources,
		})
		require.NoError(t, err)

		err = s.ApproveMaint(ctx, &entity.ApproveMaintenanceCmd{
			MaintID:               maint.ID,
			ObservedMaintRevision: maint.Revision(),
			Actor:                 testActor(),
			ConflictSnapshot: entity.ConflictsSnapshot{
				Conflicts: actualConflicts,
			},
		})
		require.ErrorIs(t, err, apperr.ErrForbiddenMaintStatusTransition)

		// The admin override skips the assignment guard, not the status guard:
		// an already-planned maintenance is not approvable by anyone.
		admin := testActor(entity.RoleAdmin)
		err = s.ApproveMaint(ctx, &entity.ApproveMaintenanceCmd{
			MaintID:               maint.ID,
			ObservedMaintRevision: maint.Revision(),
			ActorUserID:           admin.ID,
			Actor:                 admin,
			ConflictSnapshot: entity.ConflictsSnapshot{
				Conflicts: actualConflicts,
			},
		})
		require.ErrorIs(t, err, apperr.ErrForbiddenMaintStatusTransition)
	})

	t.Run("change maint revision", func(t *testing.T) {
		start, end := testdbutils.IsolatedPeriodBounds(t)

		maint := testdbutils.MakeMaint(ctx, t, maintStore, resourcesStore,
			entity.NewPeriod(start, end),
			testdbutils.WithScope(entity.MaintenanceScopeResources),
			testdbutils.WithApprover(approver.ID),
		)

		actualConflicts, err := conflictsSrv.GetConflicts(ctx, &entity.ConflictQueryCmd{
			MaintID:       maint.ID,
			PlannedPeriod: maint.PlannedPeriod,
			Scope:         maint.Scope,
			ResourceIDs:   maint.Resources,
		})
		require.NoError(t, err)

		err = s.UpdateMaint(ctx, &entity.UpdateMaintenanceCmd{
			MaintID: maint.ID,
		})
		require.NoError(t, err)

		err = s.ApproveMaint(ctx, &entity.ApproveMaintenanceCmd{
			MaintID:               maint.ID,
			ObservedMaintRevision: maint.Revision(),
			ActorUserID:           maint.ApproverUserID,
			Actor:                 approver,
			ConflictSnapshot: entity.ConflictsSnapshot{
				Conflicts: actualConflicts,
			},
		})
		require.ErrorIs(t, err, apperr.ErrMaintChangedSincePreview)
	})

	t.Run("change conflicts fingerprint", func(t *testing.T) {
		start, end := testdbutils.IsolatedPeriodBounds(t)

		maint := testdbutils.MakeMaint(ctx, t, maintStore, resourcesStore,
			entity.NewPeriod(start, end),
			testdbutils.WithScope(entity.MaintenanceScopeResources),
			testdbutils.WithApprover(approver.ID),
		)

		actualConflicts, err := conflictsSrv.GetConflicts(ctx, &entity.ConflictQueryCmd{
			MaintID:       maint.ID,
			PlannedPeriod: maint.PlannedPeriod,
			Scope:         maint.Scope,
			ResourceIDs:   maint.Resources,
		})
		require.NoError(t, err)

		// conflict maint with global scope
		testdbutils.MakeMaint(ctx, t, maintStore, resourcesStore,
			entity.NewPeriod(start, end),
			testdbutils.WithStatus(entity.MaintenanceStatusPlanned),
			testdbutils.WithScope(entity.MaintenanceScopeGlobal),
		)

		err = s.ApproveMaint(ctx, &entity.ApproveMaintenanceCmd{
			MaintID:               maint.ID,
			ObservedMaintRevision: maint.Revision(),
			ActorUserID:           maint.ApproverUserID,
			Actor:                 approver,
			ConflictSnapshot: entity.ConflictsSnapshot{
				Conflicts: actualConflicts,
			},
		})
		require.ErrorIs(t, err, apperr.ErrConflictsChangedSincePreview)
	})
}

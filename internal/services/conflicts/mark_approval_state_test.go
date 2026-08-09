package conflicts

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/entity"
)

var (
	maintID  = uuid.MustParse("dddddddd-0000-0000-0000-00000000000d")
	knownID  = uuid.MustParse("aaaaaaaa-0000-0000-0000-000000000001")
	freshID  = uuid.MustParse("bbbbbbbb-0000-0000-0000-000000000002")
	baseTime = time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
)

func conflict(id uuid.UUID, start time.Time) *entity.ConflictWithResources {
	return &entity.ConflictWithResources{
		Conflict: &entity.Conflict{
			MaintenanceID: id,
			Title:         "neighbor",
			OverlapStart:  start,
			OverlapEnd:    start.Add(time.Hour),
			Scope:         entity.MaintenanceScopeGlobal,
		},
	}
}

func TestMarkApprovalState(t *testing.T) {
	ctx := context.Background()

	t.Run("splits live conflicts into reviewed and new", func(t *testing.T) {
		svc, mocks := initService(t)
		mocks.snapshots.EXPECT().GetSnapshots(gomock.Any(), maintID).
			Return([]*entity.ConflictWithResources{conflict(knownID, baseTime)}, nil)

		live := []*entity.ConflictWithResources{
			conflict(knownID, baseTime),
			conflict(freshID, baseTime),
		}

		got := svc.MarkApprovalState(ctx, maintID, live)

		require.Len(t, got, 2)
		require.False(t, got[0].KnownAtApproval, "the unreviewed conflict must lead")
		require.Equal(t, freshID, got[0].MaintenanceID)
		require.True(t, got[1].KnownAtApproval)
	})

	t.Run("an empty snapshot leaves every conflict new", func(t *testing.T) {
		svc, mocks := initService(t)
		mocks.snapshots.EXPECT().GetSnapshots(gomock.Any(), maintID).
			Return(nil, nil)

		got := svc.MarkApprovalState(ctx, maintID, []*entity.ConflictWithResources{conflict(knownID, baseTime)})

		require.Len(t, got, 1)
		require.False(t, got[0].KnownAtApproval)
	})

	t.Run("an unreadable snapshot degrades instead of failing the card", func(t *testing.T) {
		svc, mocks := initService(t)
		mocks.snapshots.EXPECT().GetSnapshots(gomock.Any(), maintID).
			Return(nil, errors.New("snapshot store is down"))

		// Deliberately more than one conflict, in reverse order: the response
		// order is part of the contract on the failure path too, and a single
		// conflict could not tell a sorted result from an unsorted one.
		live := []*entity.ConflictWithResources{
			conflict(freshID, baseTime.Add(time.Hour)),
			conflict(knownID, baseTime),
		}

		got := svc.MarkApprovalState(ctx, maintID, live)

		require.Len(t, got, 2, "live conflicts survive a snapshot failure")
		require.False(t, got[0].KnownAtApproval, "degradation is alarming, never reassuring")
		require.False(t, got[1].KnownAtApproval)
		require.Equal(t, knownID, got[0].MaintenanceID, "the earlier overlap still leads")
	})

	t.Run("a canceled request degrades like any other failure", func(t *testing.T) {
		svc, mocks := initService(t)
		mocks.snapshots.EXPECT().GetSnapshots(gomock.Any(), maintID).
			Return(nil, fmt.Errorf("query aborted: %w", context.Canceled))

		got := svc.MarkApprovalState(ctx, maintID, []*entity.ConflictWithResources{conflict(knownID, baseTime)})

		// Cancellation is singled out only to keep it out of the error log, which
		// a test cannot see. The response shape is what stays pinned here.
		require.Len(t, got, 1)
		require.False(t, got[0].KnownAtApproval)
	})

	t.Run("no live conflicts means no snapshot read at all", func(t *testing.T) {
		svc, _ := initService(t)

		got := svc.MarkApprovalState(ctx, maintID, nil)

		require.Empty(t, got)
		require.NotNil(t, got, "an empty array, never null")
	})
}

package maint

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

func TestMaintLifecycleEvent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		from, to  entity.MaintenanceStatus
		wantEvent entity.NotifyEventKind
		wantOK    bool
	}{
		{
			name:      "planned -> in_progress = started",
			from:      entity.MaintenanceStatusPlanned,
			to:        entity.MaintenanceStatusInProgress,
			wantEvent: entity.NotifyEventMaintStarted,
			wantOK:    true,
		},
		{
			name:      "in_progress -> canceled = canceled event",
			from:      entity.MaintenanceStatusInProgress,
			to:        entity.MaintenanceStatusCancelled,
			wantEvent: entity.NotifyEventMaintCancelled,
			wantOK:    true,
		},
		{
			name:      "planned -> canceled = canceled event (auto-cancel / manual)",
			from:      entity.MaintenanceStatusPlanned,
			to:        entity.MaintenanceStatusCancelled,
			wantEvent: entity.NotifyEventMaintCancelled,
			wantOK:    true,
		},
		{
			name:      "in_progress -> completed = completed",
			from:      entity.MaintenanceStatusInProgress,
			to:        entity.MaintenanceStatusCompleted,
			wantEvent: entity.NotifyEventMaintCompleted,
			wantOK:    true,
		},
		{
			name:   "draft -> canceled = no event",
			from:   entity.MaintenanceStatusDraft,
			to:     entity.MaintenanceStatusCancelled,
			wantOK: false,
		},
		{
			name:   "draft -> planned (approve) = no event",
			from:   entity.MaintenanceStatusDraft,
			to:     entity.MaintenanceStatusPlanned,
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			event, ok := maintLifecycleEvent(tt.from, tt.to)
			require.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				require.Equal(t, tt.wantEvent, event)
			}
		})
	}
}

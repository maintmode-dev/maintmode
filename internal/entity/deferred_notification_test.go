package entity

import (
	"testing"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
)

// TestDeferredNotificationIsScheduled pins the "enqueued" rule. The uuid.Nil
// case is the reason this is not a plain nil check: a reminder row can carry a
// zero task id, which means no task was ever enqueued for it. Callers rely on
// this helper rather than comparing the pointer themselves, so the rule has to
// hold here.
func TestDeferredNotificationIsScheduled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		goqueTaskID *uuid.UUID
		want        bool
	}{
		{name: "enqueued task", goqueTaskID: lo.ToPtr(uuid.New()), want: true},
		{name: "never enqueued", goqueTaskID: nil, want: false},
		{name: "zero task id is not enqueued", goqueTaskID: lo.ToPtr(uuid.Nil), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			notification := &DeferredNotification{GoqueTaskID: tt.goqueTaskID}

			require.Equal(t, tt.want, notification.IsScheduled())
		})
	}
}

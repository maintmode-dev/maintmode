package entity

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMaintClone(t *testing.T) {
	maint := &Maintenance{
		ID:     uuid.New(),
		Status: MaintenanceStatusPlanned,
	}

	clonedMaint := maint.Clone()

	require.Equal(t, maint.ID, clonedMaint.ID)
	require.Equal(t, maint.Status, clonedMaint.Status)
	require.NotSame(t, maint, clonedMaint)

	maint.Status = MaintenanceStatusCompleted
	require.Equal(t, MaintenanceStatusPlanned, clonedMaint.Status)
}

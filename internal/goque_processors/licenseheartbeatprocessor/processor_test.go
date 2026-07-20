package licenseheartbeatprocessor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

func TestNewTaskFactory(t *testing.T) {
	t.Parallel()

	task, err := NewTaskFactory()(t.Context())
	require.NoError(t, err)
	require.EqualValues(t, entity.ProcessorTaskLicenseHeartbeat, task.Type)
	require.NotEmpty(t, task.ExternalID)
}

// Replicas ticking within the same minute must produce the same external id
// (goque unique-key fan-in); adjacent minutes must not collide.
func TestLicenseHeartbeatExternalID_MinuteBucket(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 7, 17, 12, 30, 5, 0, time.UTC)
	sameMinute := time.Date(2026, 7, 17, 12, 30, 59, 0, time.UTC)
	nextMinute := time.Date(2026, 7, 17, 12, 31, 0, 0, time.UTC)

	require.Equal(t, licenseHeartbeatExternalID(base), licenseHeartbeatExternalID(sameMinute))
	require.NotEqual(t, licenseHeartbeatExternalID(base), licenseHeartbeatExternalID(nextMinute))
}

package uicalendar

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	maintmodels "github.com/ruko1202/maintmode/internal/app/api/public/maint/models"
)

func TestMaintViewTaskTrackerURL(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t)
	const link = "https://jira.example.com/browse/OPS-42"
	maint := makeMaintWithRequest(ctx, t, func(req *maintmodels.CreateDraftMaintRequest) {
		req.TaskTrackerURL = link
	})

	resp, _ := maintViewResponse(ctx, t, impl, maint.ID)
	require.NotNil(t, resp.Maintenance)
	require.Equal(t, link, resp.Maintenance.TaskTrackerURL)
}

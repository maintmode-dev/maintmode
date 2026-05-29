//go:build api

package api

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/go-openapi/strfmt"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/utils/xtime"
	"github.com/ruko1202/maintmode/test/api/client/client/maintenances"
	"github.com/ruko1202/maintmode/test/api/client/models"
)

func TestMaintenancesAPI_ApproveRejectsStaleRevision(t *testing.T) {
	ctx := context.Background()

	apiClient := setupMaintmodeTestClient()
	maintenanceID := createTestMaintenance(ctx, t, apiClient)

	getParams := maintenances.NewGetAPIV1MaintenancesIDParams().
		WithContext(ctx).
		WithID(strfmt.UUID(maintenanceID))

	getResp, err := apiClient.Maintenances.GetAPIV1MaintenancesID(getParams, nil)
	require.NoError(t, err)
	require.NotNil(t, getResp)

	staleRevision := time.Time(getResp.Payload.CreatedAt).UnixMicro()

	resource := creatResource(ctx, t, apiClient)
	updateReq := &models.ApimodelsUpdateDraftMaintRequest{
		Title:        "Revision changed maintenance",
		Description:  "Update before approval must change the revision",
		Impact:       models.ApimodelsMaintenanceImpactPartialOutage,
		Scope:        models.ApimodelsMaintenanceScopeResource,
		PlannedStart: strfmt.DateTime(xtime.UTCNow().Add(72 * time.Hour).Truncate(time.Second)),
		Resources: []*models.ApimodelsResourceRef{
			{ID: strfmt.UUID(resource.ID)},
		},
		Steps:         testMaintenanceSteps(),
		NotifyTargets: testNotifyTargets(ctx, t, apiClient),
	}

	updateParams := maintenances.NewPostAPIV1MaintenancesIDEditParams().
		WithContext(ctx).
		WithID(strfmt.UUID(maintenanceID)).
		WithRequest(updateReq)

	_, err = apiClient.Maintenances.PostAPIV1MaintenancesIDEdit(updateParams, nil)
	require.NoError(t, err)

	approveReq := &models.ApimodelsApproveDraftMaintRequest{
		ObservedMaintRevision: staleRevision,
		ConflictsSnapshot:     []*models.ApimodelsConflict{},
	}
	approveParams := maintenances.NewPostAPIV1MaintenancesIDApproveParams().
		WithContext(ctx).
		WithID(strfmt.UUID(maintenanceID)).
		WithRequest(approveReq)

	_, err = apiClient.Maintenances.PostAPIV1MaintenancesIDApprove(approveParams, nil)
	require.Error(t, err)
	require.Equal(t, http.StatusConflict, extractErrorCode(t, err))
}

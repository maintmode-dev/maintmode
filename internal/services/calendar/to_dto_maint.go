package calendar

import (
	"github.com/ruko1202/maintmode/internal/calendardto"
	"github.com/ruko1202/maintmode/internal/entity"
)

// toListDTOMaint maps a maintenance entity to its list-projection DTO. Shared by
// every listing read-path; unlike GetMaint it carries no Resources, Steps or
// Revision — listings do not load them, and an empty slice keeps the field
// serializing as [] rather than null.
func toListDTOMaint(item *entity.Maintenance) *calendardto.Maintenance {
	return &calendardto.Maintenance{
		ID:                  item.ID,
		Title:               item.Title,
		Description:         item.Description,
		PlannedPeriod:       item.PlannedPeriod,
		ActualPeriod:        item.ActualPeriod,
		Resources:           []*calendardto.MaintenanceResource{},
		Scope:               item.Scope,
		Impact:              item.Impact,
		Status:              item.Status,
		CancelReason:        item.CancelReason,
		CancelReasonComment: item.CancelReasonComment,
		CreatedAt:           item.CreatedAt,
		UpdatedAt:           item.UpdatedAt,
		CreatedByUserID:     item.CreatedByUserID,
		ApproverUserID:      item.ApproverUserID,
	}
}

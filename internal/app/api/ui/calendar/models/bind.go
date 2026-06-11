package uimodels

import (
	"time"

	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/calendardto"
	"github.com/ruko1202/maintmode/internal/entity"
)

// ToAPIUserSummary maps a resolved domain user summary to the UI shape. Nil-safe:
// a nil summary maps to nil (null).
func ToAPIUserSummary(u *entity.UserSummary) *UserSummary {
	if u == nil {
		return nil
	}

	return &UserSummary{
		ID:          u.ID,
		DisplayName: u.Name,
		Email:       u.Email,
	}
}

// ToAPICalendarEvent maps a calendar maintenance to its list event. author is the
// resolved author summary (batch-resolved by the handler); nil when absent.
func ToAPICalendarEvent(maintEvent *calendardto.Maintenance, author *entity.UserSummary) *CalendarEvent {
	return &CalendarEvent{
		ID:        maintEvent.ID,
		Title:     maintEvent.Title,
		Start:     maintEvent.PlannedPeriod.Start,
		End:       lo.FromPtr(maintEvent.PlannedPeriod.End),
		Scope:     string(maintEvent.Scope),
		Impact:    string(maintEvent.Impact),
		Status:    maintEvent.Status,
		CreatedBy: ToAPIUserSummary(author),
	}
}

// ToAPIMaintenanceView maps a calendar maintenance to its detailed view. author
// is the resolved author summary; nil when absent.
func ToAPIMaintenanceView(maintEvent *calendardto.Maintenance, author, approver *entity.UserSummary) *MaintenanceView {
	event := &MaintenanceView{
		ID:               maintEvent.ID,
		Title:            maintEvent.Title,
		Description:      maintEvent.Description,
		PlannedTimeStart: maintEvent.PlannedPeriod.Start,
		PlannedTimeEnd:   lo.FromPtr(maintEvent.PlannedPeriod.End),
		ActualTimeStart:  nil,
		ActualTimeEnd:    nil,
		Resources: lo.Map(maintEvent.Resources, func(item *calendardto.MaintenanceResource, _ int) *MaintenanceViewResource {
			return &MaintenanceViewResource{
				ID:   item.ID,
				Name: item.Name,
			}
		}),
		Scope:               string(maintEvent.Scope),
		Impact:              string(maintEvent.Impact),
		Status:              maintEvent.Status,
		CancelReason:        string(maintEvent.CancelReason),
		CancelReasonComment: maintEvent.CancelReasonComment,
		CreatedAt:           maintEvent.CreatedAt,
		UpdatedAt:           maintEvent.UpdatedAt,
		Revision:            maintEvent.Revision,
		CreatedBy:           ToAPIUserSummary(author),
		Approver:            ToAPIUserSummary(approver),
		Steps:               toAPISteps(maintEvent),
		NotifyTargets: lo.Map(maintEvent.NotifyTargets, func(item *calendardto.MaintenanceNotifyTarget, _ int) *NotifyTargetView {
			return &NotifyTargetView{
				ID:        item.ID,
				Name:      item.Name,
				Transport: string(item.Transport),
			}
		}),
	}

	if maintEvent.ActualPeriod != nil {
		event.ActualTimeStart = lo.ToPtr(maintEvent.ActualPeriod.Start)
		event.ActualTimeEnd = maintEvent.ActualPeriod.End
	}

	return event
}

func toAPISteps(maint *calendardto.Maintenance) []*MaintenanceStep {
	res := make([]*MaintenanceStep, 0, len(maint.Steps))

	start := maint.PlannedPeriod.Start
	for _, step := range maint.Steps {
		duration := time.Minute * time.Duration(step.DurationMinutes)
		end := start.Add(duration)

		res = append(res, &MaintenanceStep{
			ID:                  step.ID,
			Order:               step.Order,
			Description:         step.Description,
			RollbackDescription: step.RollbackDescription,
			Status:              string(step.Status),
			Duration:            duration.String(),
			PlannedTimeStart:    start,
			PlannedTimeEnd:      end,
		})

		// for nextStep.PlannedTimeStart = prevStep.PlannedTimeEnd
		start = end
	}
	return res
}

func ToAPIConflictView(conflict *calendardto.Conflict) *ConflictView {
	return &ConflictView{
		MaintenanceID: conflict.MaintenanceID,
		Title:         conflict.Title,
		OverlapStart:  conflict.OverlapStart,
		OverlapEnd:    conflict.OverlapEnd,
		Scope:         string(conflict.Scope),
		Resources: lo.Map(conflict.Resources, func(item *calendardto.MaintenanceResource, _ int) *MaintenanceViewResource {
			return &MaintenanceViewResource{
				ID:   item.ID,
				Name: item.Name,
			}
		}),
	}
}

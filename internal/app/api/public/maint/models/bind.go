package apimodels

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
)

func FromAPIScope(s MaintenanceScope) (entity.MaintenanceScope, error) {
	switch s {
	case MaintenanceScopeGlobal:
		return entity.MaintenanceScopeGlobal, nil
	case MaintenanceScopeResources:
		return entity.MaintenanceScopeResources, nil
	default:
		return "", fmt.Errorf("unsupported scope: %s", s)
	}
}

func FromAPIImpact(s MaintenanceImpact) (entity.MaintenanceImpact, error) {
	switch s {
	case MaintenanceImpactNone:
		return entity.MaintenanceImpactNone, nil
	case MaintenanceImpactFull:
		return entity.MaintenanceImpactFull, nil
	case MaintenanceImpactPartial:
		return entity.MaintenanceImpactPartial, nil
	default:
		return "", fmt.Errorf("unsupported impact: %s", s)
	}
}

func FromAPIMaintenanceCancelReason(s MaintenanceCancelReason) (entity.MaintenanceCancelReason, error) {
	switch s {
	case MaintenanceCancelReasonConflict:
		return entity.MaintenanceCancelReasonConflict, nil
	case MaintenanceCancelReasonIncident:
		return entity.MaintenanceCancelReasonIncident, nil
	case MaintenanceCancelReasonBusinessDecision:
		return entity.MaintenanceCancelReasonBusinessDecision, nil
	case MaintenanceCancelReasonRescheduled:
		return entity.MaintenanceCancelReasonRescheduled, nil
	case MaintenanceCancelReasonMistake:
		return entity.MaintenanceCancelReasonMistake, nil
	default:
		return "", fmt.Errorf("unsupported cancel reason: %s", s)
	}
}

func FromAPIResources(resources []*ResourceRef) []uuid.UUID {
	return lo.Map(resources, func(item *ResourceRef, _ int) uuid.UUID {
		return item.ID
	})
}

func ToAPIResources(resourceIDs []uuid.UUID) []*ResourceRef {
	return lo.Map(resourceIDs, func(id uuid.UUID, _ int) *ResourceRef {
		return &ResourceRef{ID: id}
	})
}

// ToAPIUserSummary maps a domain user to its privacy-safe API summary. It is
// nil-safe: a nil user (unknown author) maps to nil, serialized as null.
func ToAPIUserSummary(u *entity.User) *UserSummary {
	if u == nil {
		return nil
	}

	return &UserSummary{
		ID:          u.ID,
		DisplayName: u.Name,
		Email:       u.Email,
	}
}

// ToAPIUserSummaryFromSummary maps a resolved domain user summary (from the auth
// resolver) to its API shape. Nil-safe: a nil summary maps to nil (null).
func ToAPIUserSummaryFromSummary(u *entity.UserSummary) *UserSummary {
	if u == nil {
		return nil
	}

	return &UserSummary{
		ID:          u.ID,
		DisplayName: u.Name,
		Email:       u.Email,
	}
}

func ToAPIPeriod(p entity.Period) Period {
	if p.IsOpen() {
		return Period{Start: p.Start, End: nil}
	}
	return Period{Start: p.Start, End: p.End}
}

func FromAPISteps(steps []*MaintenanceStepInput) ([]*entity.MaintenanceStepInput, error) {
	res := make([]*entity.MaintenanceStepInput, 0, len(steps))
	for _, step := range steps {
		duration, err := time.ParseDuration(step.Duration)
		if err != nil {
			return nil, err
		}
		res = append(res, &entity.MaintenanceStepInput{
			Order:               step.Order,
			Description:         step.Description,
			RollbackDescription: step.RollbackDescription,
			DurationMinutes:     int64(duration.Minutes()),
		})
	}

	return res, nil
}

func toAPIStepStatus(s entity.MaintenanceStepStatus) MaintenanceStepStatus {
	switch s {
	case entity.MaintenanceStepStatusPlanned:
		return MaintenanceStepStatusPlanned
	case entity.MaintenanceStepStatusStarted:
		return MaintenanceStepStatusStarted
	case entity.MaintenanceStepStatusCompleted:
		return MaintenanceStepStatusCompleted
	case entity.MaintenanceStepStatusCanceled:
		return MaintenanceStepStatusCanceled
	default:
		return MaintenanceStepStatusUnknown
	}
}

func ToAPISteps(steps []*entity.MaintenanceStep) []*MaintenanceStep {
	return lo.Map(steps, func(item *entity.MaintenanceStep, _ int) *MaintenanceStep {
		return &MaintenanceStep{
			ID:                  item.ID,
			Order:               item.Order,
			Description:         item.Description,
			RollbackDescription: item.RollbackDescription,
			DurationMinutes:     item.DurationMinutes,
			Status:              toAPIStepStatus(item.Status),
		}
	})
}

func FromAPINotifyTargets(notification *NotifyTargets) ([]*entity.NotifyTargetInput, error) {
	if notification == nil {
		return nil, nil
	}

	targets := make([]*entity.NotifyTargetInput, 0, len(notification.ChannelIDs))
	for _, item := range notification.ChannelIDs {
		channelID, err := uuid.Parse(item)
		if err != nil {
			return nil, fmt.Errorf("invalid channel id %q: %w", item, err)
		}
		targets = append(targets, &entity.NotifyTargetInput{ChannelID: channelID})
	}
	return targets, nil
}

// ToAPINotifyTargets maps targets to the create/update response shape: the
// catalog channel uuids, round-trippable with the channel_ids request field.
func ToAPINotifyTargets(targets []*entity.NotifyTarget) *NotifyTargets {
	return &NotifyTargets{
		ChannelIDs: lo.Map(targets, func(item *entity.NotifyTarget, _ int) string {
			return item.ChannelID.String()
		}),
	}
}

// ToAPINotifyTargetViews maps catalog-resolved notify targets to detail-view
// chips.
func ToAPINotifyTargetViews(targets []*entity.NotifyTarget) []*NotifyTargetView {
	return lo.Map(targets, func(item *entity.NotifyTarget, _ int) *NotifyTargetView {
		return &NotifyTargetView{
			ID:        item.ChannelID,
			Name:      item.ChannelName,
			Transport: string(item.Transport),
		}
	})
}

// FromAPIDeferredNotifications maps the contract's deferred_notifications to
// domain inputs. It works on a flat slice and carries no "unchanged" state:
// create has none to express, and update encodes that state in the pointer
// around the slice, which its caller unwraps before mapping.
func FromAPIDeferredNotifications(notifications []*DeferredNotification) []*entity.DeferredNotificationInput {
	return lo.Map(notifications, func(item *DeferredNotification, _ int) *entity.DeferredNotificationInput {
		return &entity.DeferredNotificationInput{
			FireAt: item.FireAt,
		}
	})
}

func ToAPIDeferredNotifications(notifications []*entity.DeferredNotification) []*DeferredNotification {
	return lo.Map(notifications, func(item *entity.DeferredNotification, _ int) *DeferredNotification {
		return &DeferredNotification{
			FireAt: item.FireAt,
		}
	})
}

// ToAPIMaintenance maps a maintenance entity to its API DTO. author is the
// resolved author summary (from the auth resolver); pass nil when there is no
// author to render — the resolver supplies a labeled "Unknown user" summary
// rather than nil when auth cannot resolve the id.
func ToAPIMaintenance(m *entity.Maintenance, author, approver *entity.UserSummary) *Maintenance {
	maint := &Maintenance{
		ID:                    m.ID,
		Title:                 m.Title,
		Description:           m.Description,
		PlannedPeriod:         ToAPIPeriod(m.PlannedPeriod),
		ActualPeriod:          nil,
		Resources:             ToAPIResources(m.Resources),
		Scope:                 MaintenanceScope(m.Scope),
		Impact:                MaintenanceImpact(m.Impact),
		Status:                string(m.Status),
		CancelReason:          MaintenanceCancelReason(m.CancelReason),
		CancelReasonComment:   m.CancelReasonComment,
		CreatedAt:             m.CreatedAt,
		UpdatedAt:             m.UpdatedAt,
		CreatedBy:             ToAPIUserSummaryFromSummary(author),
		Approver:              ToAPIUserSummaryFromSummary(approver),
		Steps:                 ToAPISteps(m.Steps),
		NotifyTargets:         ToAPINotifyTargetViews(m.NotifyTargets),
		DeferredNotifications: ToAPIDeferredNotifications(m.DeferredNotifications),
	}

	if m.ActualPeriod != nil {
		maint.ActualPeriod = lo.ToPtr(ToAPIPeriod(lo.FromPtr(m.ActualPeriod)))
	}

	return maint
}

package maint

import (
	"context"
	"errors"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xvalidation"
)

func (s *Service) UpdateMaint(ctx context.Context, cmd *entity.UpdateMaintenanceCmd) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Maint.Update")
	defer span.End()

	if err := validateUpdate(ctx, cmd); err != nil {
		xlog.Error(ctx, "validation failed", xfield.Error(err))
		return err
	}

	// nil approver means "keep the current approver"; only validate a new one.
	if cmd.ApproverUserID != nil {
		if err := s.validateApprover(ctx, lo.FromPtr(cmd.ApproverUserID)); err != nil {
			return err
		}
	}

	notifyTargets, err := s.notifyTargets.ResolveNotifyTarget(ctx, cmd.MaintID, cmd.NotifyTargets)
	if err != nil {
		xlog.Error(ctx, "failed to resolve notify targets", xfield.Error(err))
		return err
	}

	return s.updateWithApply(ctx, cmd.MaintID, func(ctx context.Context, maint *entity.Maintenance) error {
		return s.applyUpdate(ctx, maint, notifyTargets, cmd)
	})
}

// applyUpdate mutates and re-persists a draft maintenance's child collections
// from the update command. Each collection is replaced only when the command
// carries it; see the per-block comments for the "unchanged vs clear" rules.
func (s *Service) applyUpdate(
	ctx context.Context,
	maint *entity.Maintenance,
	notifyTargets []*entity.NotifyTarget,
	cmd *entity.UpdateMaintenanceCmd,
) error {
	if maint.Status != entity.MaintenanceStatusDraft {
		return apperr.ForbiddenMaintStatusTransition(maint.Status, entity.MaintenanceStatusDraft)
	}

	applyValuesFromUpdateCmd(maint, notifyTargets, cmd)

	if len(cmd.Steps) > 0 {
		if err := s.replaceSteps(ctx, maint); err != nil {
			xlog.Error(ctx, "replace steps failed", xfield.Error(err))
			return err
		}
	}

	if len(cmd.Resources) > 0 || lo.FromPtr(cmd.Scope) == entity.MaintenanceScopeGlobal {
		if err := s.replaceResources(ctx, maint); err != nil {
			xlog.Error(ctx, "replace resources failed", xfield.Error(err))
			return err
		}
	}

	// A maintenance must always keep >=1 notify target (enforced by
	// validateUpdate's Required-equivalent on create and the cap
	// here): an empty NotifyTargets means "not changing targets",
	// not "clear them". So we only replace when targets are given.
	if len(cmd.NotifyTargets) > 0 {
		if err := s.notifyTargets.Replace(ctx, maint); err != nil {
			xlog.Error(ctx, "replace notify target failed", xfield.Error(err))
			return err
		}
	}

	// Empty/nil means "not changing reminders" (same convention as
	// NotifyTargets above), so we only replace when reminders are given.
	// Edits are only allowed while a maintenance is a draft, so no goque
	// tasks exist yet — they're enqueued on approve. Persisting here is
	// enough; no re-enqueue needed (see deferrednotifications.Replace).
	if len(cmd.DeferredNotifications) > 0 {
		if err := s.deferred.Replace(ctx, maint.ID, maint.DeferredNotifications); err != nil {
			xlog.Error(ctx, "replace deferred notifications failed", xfield.Error(err))
			return err
		}
	}

	return nil
}

func applyValuesFromUpdateCmd(
	maint *entity.Maintenance,
	notifyTargets []*entity.NotifyTarget,
	cmd *entity.UpdateMaintenanceCmd,
) {
	if cmd.Title != nil {
		maint.Title = lo.FromPtr(cmd.Title)
	}

	if cmd.Description != nil {
		maint.Description = lo.FromPtr(cmd.Description)
	}

	if len(cmd.Steps) > 0 {
		maint.Steps = stepsFromCmd(cmd.Steps, entity.MaintenanceStepStatusPlanned)
		maint.PlannedPeriod = recalculatePlannedPeriod(maint.PlannedPeriod.Start, maint.Steps)
	}

	if cmd.PlannedStart != nil {
		currentDuration := maint.PlannedPeriod.Duration()

		newStart := lo.FromPtr(cmd.PlannedStart)
		newEnd := newStart.Add(currentDuration)

		maint.PlannedPeriod = entity.NewPeriod(newStart, newEnd)
	}

	if cmd.Scope != nil {
		maint.Scope = lo.FromPtr(cmd.Scope)
	}

	if cmd.Impact != nil {
		maint.Impact = lo.FromPtr(cmd.Impact)
	}

	if cmd.ApproverUserID != nil {
		maint.ApproverUserID = lo.FromPtr(cmd.ApproverUserID)
	}

	if len(cmd.Resources) > 0 {
		maint.Resources = cmd.Resources
	}

	if len(notifyTargets) > 0 {
		maint.NotifyTargets = notifyTargets
	}

	if len(cmd.DeferredNotifications) > 0 {
		maint.DeferredNotifications = deferredNotificationsFromCmd(cmd.MaintID, cmd.DeferredNotifications)
	}

	maint.Normalize()
}

func (s *Service) replaceResources(ctx context.Context, maint *entity.Maintenance) error {
	if err := s.maintStore.DeleteResources(ctx, maint.ID); err != nil {
		xlog.Error(ctx, "failed to delete resources", xfield.Error(err))
		return err
	}

	if err := s.maintStore.AddResources(ctx, maint.ID, maint.Resources); err != nil {
		xlog.Error(ctx, "failed to add resources", xfield.Error(err))
		return err
	}
	return nil
}

func (s *Service) replaceSteps(ctx context.Context, maint *entity.Maintenance) error {
	if err := s.maintStore.DeleteSteps(ctx, maint.ID); err != nil {
		xlog.Error(ctx, "failed to delete steps", xfield.Error(err))
		return err
	}

	_, err := s.maintStore.AddSteps(ctx, maint.ID, maint.Steps)
	if err != nil {
		xlog.Error(ctx, "failed to add steps", xfield.Error(err))
		return err
	}
	return nil
}

func validateUpdate(ctx context.Context, cmd *entity.UpdateMaintenanceCmd) error {
	return validation.ValidateStructWithContext(ctx, cmd,
		validation.Field(&cmd.Title, validation.NilOrNotEmpty),
		validation.Field(&cmd.Description, validation.NilOrNotEmpty),
		validation.Field(&cmd.ApproverUserID, validation.NilOrNotEmpty,
			validation.When(cmd.ApproverUserID != nil, validation.By(xvalidation.UUIDNotNil)),
		),
		validation.Field(&cmd.PlannedStart, validation.NilOrNotEmpty),
		validation.Field(&cmd.Scope, validation.NilOrNotEmpty),
		validation.Field(&cmd.Impact, validation.NilOrNotEmpty),

		// validate only if changed
		validation.Field(&cmd.Resources, validation.Each(validation.By(xvalidation.UUIDNotNil))),
		validation.Field(&cmd.Steps, validation.Each(validation.WithContext(validateStepInput))),
		validation.Field(&cmd.NotifyTargets,
			validation.Length(0, maxNotifyTargets),
			validation.Each(validation.WithContext(validateNotifyTargetsInput)),
		),
		validation.Field(&cmd.DeferredNotifications,
			validation.Length(0, maxDeferredNotifications),
			validation.Each(validation.WithContext(validateDeferredNotificationInput)),
		),
	)
}

func (s *Service) updateWithApply(ctx context.Context, maintID uuid.UUID, apply func(ctx context.Context, maint *entity.Maintenance) error) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Maint.updateWithApply")
	defer span.End()

	return s.runUpdateWithApply(ctx, func(ctx context.Context, fn func(context.Context) error) error {
		return s.txManager.WithinTx(ctx, fn)
	}, maintID, apply)
}

// updateWithApplySerializable is updateWithApply on a SERIALIZABLE transaction
// with retry on serialization conflicts. Use it for transitions whose
// correctness depends on a stable read of *other* rows inside the tx (approve,
// which recomputes the conflict set), not just a lock on this maintenance.
func (s *Service) updateWithApplySerializable(ctx context.Context, maintID uuid.UUID, apply func(ctx context.Context, maint *entity.Maintenance) error) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Maint.updateWithApplySerializable")
	defer span.End()

	return s.runUpdateWithApply(ctx, s.txManager.WithinSerializableTx, maintID, apply)
}

// errSkipUpdate is a sentinel an apply func may return to abort the update as a
// deliberate no-op: the row read under the lock did not warrant a change. The tx
// is rolled back (so no write, no lifecycle dispatch) and the sentinel is
// propagated to the caller, but it is NOT logged as a failure. Callers should
// treat it as "nothing to do", typically with errors.Is.
var errSkipUpdate = errors.New("maint update skipped: precondition not met")

func (s *Service) runUpdateWithApply(
	ctx context.Context,
	withinTx func(context.Context, func(context.Context) error) error,
	maintID uuid.UUID,
	apply func(ctx context.Context, maint *entity.Maintenance) error,
) error {
	var (
		prev    *entity.Maintenance
		current *entity.Maintenance
	)
	err := withinTx(ctx, func(ctx context.Context) error {
		maint, err := s.maintStore.GetMaintForUpdate(ctx, maintID)
		if err != nil {
			xlog.Error(ctx, "failed to get maint for update", xfield.Error(err))
			return err
		}
		prev = maint.Clone()
		current = maint

		err = apply(ctx, maint)
		if err != nil {
			return err
		}

		err = s.maintStore.UpdateMaint(ctx, maint)
		if err != nil {
			xlog.Error(ctx, "failed to update maint", xfield.Error(err))
			return err
		}

		return nil
	})
	if err != nil {
		// errSkipUpdate is a deliberate no-op, not a failure: pass it through
		// unlogged so the caller can recognize the skip.
		if !errors.Is(err, errSkipUpdate) {
			xlog.Error(ctx, "failed to update maint", xfield.Error(err))
		}
		return err
	}

	s.dispatchMaintLifecycle(ctx, prev, current)
	return nil
}

func (s *Service) dispatchMaintLifecycle(ctx context.Context, prev, current *entity.Maintenance) {
	eventType, ok := maintLifecycleEvent(prev.Status, current.Status)
	if !ok {
		return
	}

	if eventType == entity.NotifyEventMaintCancelled {
		// Stop any pending reminders for the now-canceled maintenance (best-effort,
		// post-commit). A draft has none, but the call is a harmless no-op then.
		_ = s.deferred.Cancel(ctx, current.ID)
	}

	s.notifier.NotifyMaintLifecycle(ctx, eventType, current)
}

// maintLifecycleEvent maps a status transition to the lifecycle notification it
// should emit. The bool is false when the transition has no associated
// notification (draft edits, approve, draft->canceled). draft->canceled is silent
// on purpose: a draft was never announced to notify targets. Kept pure so the
// transition matrix is unit-testable without a notifier.
func maintLifecycleEvent(from, to entity.MaintenanceStatus) (entity.NotifyEventKind, bool) {
	switch {
	case from == entity.MaintenanceStatusPlanned && to == entity.MaintenanceStatusInProgress:
		return entity.NotifyEventMaintStarted, true
	case from == entity.MaintenanceStatusPlanned && to == entity.MaintenanceStatusCancelled:
		return entity.NotifyEventMaintCancelled, true
	case from == entity.MaintenanceStatusInProgress && to == entity.MaintenanceStatusCancelled:
		return entity.NotifyEventMaintCancelled, true
	case from == entity.MaintenanceStatusInProgress && to == entity.MaintenanceStatusCompleted:
		return entity.NotifyEventMaintCompleted, true
	default:
		return "", false
	}
}

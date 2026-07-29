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
	"github.com/ruko1202/maintmode/internal/audit"
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

	prev, current, err := s.updateWithApply(ctx, cmd.MaintID, func(ctx context.Context, maint *entity.Maintenance) error {
		return s.applyUpdate(ctx, maint, notifyTargets, cmd)
	})
	if err != nil {
		xlog.Error(ctx, "failed to update maint", xfield.Error(err))
		return err
	}

	// Audit only a material change: an update whose diff is empty (e.g.
	// collections re-supplied identically) is not worth a row.
	if changes := maintUpdateChanges(prev, current, cmd); len(changes) > 0 {
		s.publishAudit(ctx, audit.MaintUpdated{Actor: cmd.Actor, Maint: current, Changes: changes})
	}

	return nil
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

	// Unlike NotifyTargets above, reminders are tri-state: nil means "not
	// changing reminders", while a supplied set — including an empty one —
	// replaces them, so an empty set clears them. This gate is paired with the
	// one in applyValuesFromUpdateCmd, which stages what Replace persists; they
	// must agree, or clearing would rewrite the previous set instead of
	// deleting it.
	// Edits are only allowed while a maintenance is a draft, so no goque
	// tasks exist yet — they're enqueued on approve. Persisting here is
	// enough; no re-enqueue needed (see deferrednotifications.Replace).
	if cmd.DeferredNotifications != nil {
		if err := s.deferred.Replace(ctx, maint.ID, maint.DeferredNotifications); err != nil {
			xlog.Error(ctx, "replace deferred notifications failed", xfield.Error(err))
			return err
		}
	}

	// Mentions are tri-state like the reminders above: nil means "not changing
	// mentions", a supplied set — empty included — replaces them. Paired with the
	// gate in applyValuesFromUpdateCmd, which stages what this block persists;
	// they must agree, or clearing would rewrite the previous set instead of
	// deleting it.
	if cmd.Mentions != nil {
		if err := s.replaceMentions(ctx, maint); err != nil {
			xlog.Error(ctx, "replace mentions failed", xfield.Error(err))
			return err
		}
	}

	return nil
}

// replaceMentions swaps the whole mention set under the maintenance row lock
// held by runUpdateWithApply, so the delete and the re-insert are one atomic
// replacement rather than a window with no mentions.
//
// Neither failure is logged here: the caller already logs the returned error,
// as it does for the notify targets and reminders beside it.
func (s *Service) replaceMentions(ctx context.Context, maint *entity.Maintenance) error {
	if err := s.maintStore.DeleteMentions(ctx, maint.ID); err != nil {
		return err
	}

	return s.maintStore.AddMentions(ctx, maint.ID, maint.Mentions)
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

	// Paired with the Replace gate in applyUpdate: a supplied set (empty
	// included) is staged here and persisted there. An empty set stages an
	// empty collection, which is what makes clearing delete rather than
	// rewrite the previous reminders.
	if cmd.DeferredNotifications != nil {
		maint.DeferredNotifications = deferredNotificationsFromCmd(cmd.MaintID, lo.FromPtr(cmd.DeferredNotifications))
	}

	// Paired with the replace gate in applyUpdate, on the same rule as the
	// reminders above: a supplied set (empty included) is staged here and
	// persisted there, so an empty set stages an empty collection and clearing
	// deletes rather than rewrites.
	if cmd.Mentions != nil {
		maint.Mentions = lo.Uniq(lo.Map(lo.FromPtr(cmd.Mentions), func(m *entity.MentionInput, _ int) uuid.UUID { return m.UserID }))
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
	if err := validation.ValidateStructWithContext(ctx, cmd,
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
		// Length works on the pointer field directly — LengthRule dereferences
		// via Indirect. Each does not, so it runs separately below.
		validation.Field(&cmd.DeferredNotifications, validation.Length(0, maxDeferredNotifications)),
		validation.Field(&cmd.Mentions, validation.Length(minMentions, maxMentions)),
	); err != nil {
		return err
	}

	// Each must receive the dereferenced slice: EachRule switches on
	// reflect.Kind and rejects a pointer as "must be an iterable", which would
	// also mask the per-element rules (a missing fire_at would stop being
	// caught). A nil pointer yields an empty slice, so "unchanged" validates
	// trivially.
	if err := validation.ValidateWithContext(ctx, lo.FromPtr(cmd.DeferredNotifications),
		validation.Each(validation.WithContext(validateDeferredNotificationInput))); err != nil {
		return err
	}

	// Same dereference requirement as the reminders above, plus the cross-element
	// uniqueness rule ozzo cannot express per element.
	if err := validation.ValidateWithContext(ctx, lo.FromPtr(cmd.Mentions),
		validation.Each(validation.WithContext(validateMentionInput)),
	); err != nil {
		return validation.Errors{"Mentions": err}
	}

	return nil
}

func (s *Service) updateWithApply(ctx context.Context, maintID uuid.UUID, apply func(ctx context.Context, maint *entity.Maintenance) error) (prev, current *entity.Maintenance, err error) {
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
func (s *Service) updateWithApplySerializable(ctx context.Context, maintID uuid.UUID, apply func(ctx context.Context, maint *entity.Maintenance) error) (prev, current *entity.Maintenance, err error) {
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

// runUpdateWithApply runs apply against the locked maintenance inside withinTx,
// persists it, and on success dispatches the lifecycle notification (derived
// from the status transition — it needs no caller input). It returns the pre-
// and post-mutation snapshots so the caller can publish its own audit action
// explicitly; this keeps the transactional helper free of any audit knowledge.
// On error (including errSkipUpdate) prev/current are nil and no dispatch runs.
func (s *Service) runUpdateWithApply(
	ctx context.Context,
	withinTx func(context.Context, func(context.Context) error) error,
	maintID uuid.UUID,
	apply func(ctx context.Context, maint *entity.Maintenance) error,
) (prev, current *entity.Maintenance, err error) {
	err = withinTx(ctx, func(ctx context.Context) error {
		maint, err := s.maintStore.GetMaintForUpdate(ctx, maintID)
		if err != nil {
			xlog.Error(ctx, "failed to get maint for update", xfield.Error(err))
			return err
		}
		prev = maint.Clone()
		current = maint

		if err := apply(ctx, maint); err != nil {
			return err
		}

		if err := s.maintStore.UpdateMaint(ctx, maint); err != nil {
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
		return nil, nil, err
	}

	// Notify is derived from the status transition, so the helper owns it (no
	// caller input needed). Audit, by contrast, is an explicit per-mutation
	// action: the caller publishes it from the returned prev/current.
	s.dispatchMaintLifecycle(ctx, prev, current)
	return prev, current, nil
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

// maintUpdateChanges diffs the pre- and post-update maintenance into the
// structured before/after entries the audit metadata carries (RUK-182). Scalar
// fields record old → new; collection fields (steps, targets) record only a
// changed flag (empty old/new) since element-level diffs are noisy.
//
// Scalars are diffed prev→current (those columns are loaded by
// GetMaintForUpdate). Collections (steps/resources/notify_targets) are NOT
// loaded onto prev, so they cannot be diffed by content here; instead the
// changed flag mirrors the update's own replace rule — a collection is recorded
// as changed exactly when the command supplied a replacement set (the same
// condition applyUpdate uses to replace it — non-empty for steps, resources
// and notify targets; non-nil for the tri-state deferred notifications). This
// avoids an extra hydrating read on every draft edit and reports "the update
// replaced this collection", which is what applyUpdate actually did.
func maintUpdateChanges(prev, current *entity.Maintenance, cmd *entity.UpdateMaintenanceCmd) []entity.AuditFieldChange {
	var changes []entity.AuditFieldChange

	addScalar := func(field, old, neu string) {
		if old != neu {
			changes = append(changes, entity.AuditFieldChange{Field: field, Old: old, New: neu})
		}
	}

	addScalar("title", prev.Title, current.Title)
	addScalar("description", prev.Description, current.Description)
	addScalar("planned_period", prev.PlannedPeriod.String(), current.PlannedPeriod.String())
	addScalar("scope", string(prev.Scope), string(current.Scope))
	addScalar("impact", string(prev.Impact), string(current.Impact))
	addScalar("approver", prev.ApproverUserID.String(), current.ApproverUserID.String())

	addCollection := func(field string, supplied bool) {
		if supplied {
			changes = append(changes, entity.AuditFieldChange{Field: field})
		}
	}
	addCollection("steps", len(cmd.Steps) > 0)
	addCollection("resources", len(cmd.Resources) > 0 || lo.FromPtr(cmd.Scope) == entity.MaintenanceScopeGlobal)
	addCollection("notify_targets", len(cmd.NotifyTargets) > 0)
	// Mirrors applyUpdate's tri-state gate: a supplied set is a change even when
	// empty. Clearing hard-deletes the reminders, so this audit entry is the
	// only record that it happened.
	addCollection("deferred_notifications", cmd.DeferredNotifications != nil)
	// Same tri-state gate as applyUpdate: a supplied set is a change even when
	// empty, and clearing hard-deletes the mentions, so this entry is the only
	// record that it happened.
	addCollection("mentions", cmd.Mentions != nil)

	return changes
}

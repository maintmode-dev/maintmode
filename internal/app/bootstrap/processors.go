package bootstrap

import (
	"fmt"
	"time"

	"github.com/ruko1202/goque"

	"github.com/ruko1202/maintmode/internal/goque_processors/asyncsenderprocessor"
	"github.com/ruko1202/maintmode/internal/goque_processors/auditprocessor"
	"github.com/ruko1202/maintmode/internal/goque_processors/auditpruneprocessor"
	"github.com/ruko1202/maintmode/internal/goque_processors/autocancelprocessor"
	"github.com/ruko1202/maintmode/internal/goque_processors/reminderprocessor"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
)

// NewTaskProcessors builds the single goque worker for the maintmode process and
// registers every task type the merged process owns (RUK-194): messaging.send,
// maint.reminder, maint.auto.cancel (+ cron), invitation.email, audit.write and
// audit.prune (+ cron). All eight are registered on one registrar, and verify()
// runs once at the end to assert the registered set matches the canonical
// entity.ActiveProcessorTaskTypes.
func NewTaskProcessors(
	cfg config.TaskProcessorConfig,
	stores *Stores,
	services *Services,
) (*goque.Goque, error) {
	goq := goque.NewGoque(stores.taskStorage)
	reg := newProcessorRegistrar(goq)

	reg.RegisterProcessor(
		entity.ProcessorTaskMessagingSend,
		asyncsenderprocessor.NewTaskProcessor(services.TransportResolver),
		goque.WithWorkersCount(cfg.Messaging.Workers),
		goque.WithTaskProcessingMaxAttempts(cfg.Messaging.MaxAttempts),
	)

	// maint.reminder tasks resolve the maintenance's current notify targets and
	// render the reminder at fire time, so they share the maint store + notifier
	// rather than carrying a pre-rendered payload.
	reg.RegisterProcessor(
		entity.ProcessorTaskMaintReminder,
		reminderprocessor.NewTaskProcessor(stores.Maintenances, services.Notifier),
		goque.WithWorkersCount(cfg.Messaging.Workers),
		goque.WithTaskProcessingMaxAttempts(cfg.Messaging.MaxAttempts),
	)

	// maint.auto.cancel: a periodic cron job enqueues one task per tick (carrying
	// the grace threshold + batch limit from config) and this processor sweeps
	// not-started maintenances (draft or planned) overdue past the grace window. One
	// worker is enough — the sweep is a single bounded batch and must not run
	// concurrently with itself.
	//
	// On multi-replica deploys (the Caddyfile supports `--scale maintmode=N`) every
	// replica ticks the schedule, but the minute-bucketed external id (see
	// autocancelprocessor.NewTaskFactory) collapses them to one *enqueued* task per
	// minute via the goque (type, external_id) unique constraint. NOTE: goque
	// v0.8.9 logs each losing insert at ERROR ("failed to add task" /
	// "failed to add periodic job task to queue") and does not retry — so with N
	// replicas expect ~2×(N-1) benign duplicate-key ERROR lines per minute. They are
	// correctness-neutral; suppressing them requires an upstream goque change.
	autoCancelCfg := cfg.MaintAutoCancel
	reg.RegisterProcessor(
		entity.ProcessorTaskMaintAutoCancel,
		autocancelprocessor.NewTaskProcessor(services.Maint),
		goque.WithWorkersCount(1),
		goque.WithTaskProcessingMaxAttempts(cfg.Messaging.MaxAttempts),
	)

	autoCancelJob, err := goque.NewCronJob(
		entity.ProcessorTaskMaintAutoCancelCron,
		autoCancelCfg.CronSpec,
		time.UTC,
		autocancelprocessor.NewTaskFactory(autoCancelCfg.Threshold, autoCancelCfg.BatchLimit),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to build maint-auto-cancel cron job: %w", err)
	}
	reg.RegisterPeriodicJob(autoCancelJob)

	// invitation.email: invitation emails enqueued via the outbox deliver through
	// the same notify-transport registry the invitation service targets.
	//
	// It is a distinct task type from messaging.send on purpose: although both are
	// now drained by this one process, keeping invitation emails on their own type
	// preserves the registry-routing boundary and lets the owner map stay explicit.
	reg.RegisterProcessor(
		entity.ProcessorTaskInvitationEmailSend,
		asyncsenderprocessor.NewTaskProcessor(services.TransportResolver),
		goque.WithWorkersCount(cfg.Messaging.Workers),
		goque.WithTaskProcessingMaxAttempts(cfg.Messaging.MaxAttempts),
	)

	// audit.write: domain events published via auditpublisher.Publish land here as
	// rendered audit snapshots (RUK-179). The processor writes audit_log after
	// commit, outside any tx. An idempotent INSERT (ON CONFLICT event_id) makes
	// retries safe.
	reg.RegisterProcessor(
		entity.ProcessorTaskAuditWrite,
		auditprocessor.NewTaskProcessor(services.Audit),
		goque.WithWorkersCount(cfg.Messaging.Workers),
		goque.WithTaskProcessingMaxAttempts(cfg.Messaging.MaxAttempts),
	)

	// audit.prune: a daily cron job enqueues one prune task (carrying the retention
	// window + batch limit from config) and this processor deletes audit_log rows
	// older than the window in bounded batches. One worker is enough — the sweep is
	// a single drained DELETE loop and must not run concurrently with itself.
	//
	// On multi-replica deploys every replica ticks the schedule, but the
	// day-bucketed external id (see auditpruneprocessor.NewTaskFactory) collapses
	// them to one enqueued task per day via the goque (type, external_id) unique
	// constraint. As with maint.auto.cancel, goque v0.8.9 logs each losing insert at
	// ERROR and does not retry; the duplicates are correctness-neutral.
	auditPruneCfg := cfg.AuditPrune
	reg.RegisterProcessor(
		entity.ProcessorTaskAuditPrune,
		auditpruneprocessor.NewTaskProcessor(services.Audit),
		goque.WithWorkersCount(1),
		goque.WithTaskProcessingMaxAttempts(cfg.Messaging.MaxAttempts),
	)

	auditPruneJob, err := goque.NewCronJob(
		entity.ProcessorTaskAuditPruneCron,
		auditPruneCfg.CronSpec,
		time.UTC,
		auditpruneprocessor.NewTaskFactory(auditPruneCfg.Retention, auditPruneCfg.BatchLimit),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to build audit-prune cron job: %w", err)
	}
	reg.RegisterPeriodicJob(auditPruneJob)

	if err := reg.verify(); err != nil {
		return nil, err
	}

	return goq, nil
}

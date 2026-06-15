package bootstrap

import (
	"fmt"
	"time"

	"github.com/ruko1202/goque"

	"github.com/ruko1202/maintmode/internal/goque_processors/asyncsenderprocessor"
	"github.com/ruko1202/maintmode/internal/goque_processors/auditpruneprocessor"
	"github.com/ruko1202/maintmode/internal/goque_processors/autocancelprocessor"
	"github.com/ruko1202/maintmode/internal/goque_processors/reminderprocessor"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
)

func NewTaskProcessors(
	cfg config.TaskProcessorConfig,
	stores *Stores,
	services *Services,
	gateways *Gateways,
) (*goque.Goque, error) {
	goq := goque.NewGoque(stores.taskStorage)
	goq.RegisterProcessor(
		entity.ProcessorTaskMessagingSend,
		asyncsenderprocessor.NewTaskProcessor(gateways.NotifyTransportRegistry),
		goque.WithWorkersCount(cfg.Messaging.Workers),
		goque.WithTaskProcessingMaxAttempts(cfg.Messaging.MaxAttempts),
	)

	// maint.reminder tasks resolve the maintenance's current notify targets and
	// render the reminder at fire time, so they share the maint store + notifier
	// rather than carrying a pre-rendered payload.
	goq.RegisterProcessor(
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
	goq.RegisterProcessor(
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
	goq.RegisterPeriodicJob(autoCancelJob)

	return goq, nil
}

// NewAuthTaskProcessors builds the goque worker for the auth binary. It drains
// invitation.email tasks (invitation emails enqueued via the outbox) against the
// email registry the invitation service targets, and runs the audit-retention
// prune (the auth binary owns the audit store).
//
// It deliberately registers invitation.email, NOT messaging.send: the maintmode
// binary also polls the shared goque_task table for messaging.send (Slack/Telegram
// maint notifications) with a different registry, so invitation emails get their
// own task type to guarantee only this binary delivers them. maint.reminder is
// likewise maintmode-only and not registered here.
func NewAuthTaskProcessors(
	cfg config.TaskProcessorConfig,
	stores *AuthStores,
	services *AuthServices,
	gateways *AuthGateways,
) (*goque.Goque, error) {
	goq := goque.NewGoque(stores.taskStorage)
	goq.RegisterProcessor(
		entity.ProcessorTaskInvitationEmailSend,
		asyncsenderprocessor.NewTaskProcessor(gateways.NotifyTransportRegistry),
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
	goq.RegisterProcessor(
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
	goq.RegisterPeriodicJob(auditPruneJob)

	return goq, nil
}

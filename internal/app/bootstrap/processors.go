package bootstrap

import (
	"cmp"
	"fmt"
	"time"

	"github.com/ruko1202/goque"

	"github.com/ruko1202/maintmode/internal/goque_processors/asyncsenderprocessor"
	"github.com/ruko1202/maintmode/internal/goque_processors/auditprocessor"
	"github.com/ruko1202/maintmode/internal/goque_processors/auditpruneprocessor"
	"github.com/ruko1202/maintmode/internal/goque_processors/autocancelprocessor"
	"github.com/ruko1202/maintmode/internal/goque_processors/invitationpruneprocessor"
	"github.com/ruko1202/maintmode/internal/goque_processors/invitationrotateprocessor"
	"github.com/ruko1202/maintmode/internal/goque_processors/licenseheartbeatprocessor"
	"github.com/ruko1202/maintmode/internal/goque_processors/reminderprocessor"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
)

// NewTaskProcessors builds the single goque worker for the maintmode process and
// registers every task type the merged process owns: maint.reminder,
// maint.auto.cancel (+ cron), invitation.email, audit.write and audit.prune
// (+ cron), plus license.heartbeat (+ cron) when SaaS license mode is enabled.
// Everything is registered on one registrar, and verify() runs once at the end to
// assert the registered set matches entity.ExpectedProcessorTaskTypes for this
// process's toggles.
func NewTaskProcessors(
	cfg config.TaskProcessorConfig,
	licenseCfg config.LicenseConfig,
	stores *Stores,
	services *Services,
) (*goque.Goque, error) {
	goq := goque.NewGoque(stores.taskStorage)
	reg := newProcessorRegistrar(goq)

	// maint.reminder tasks resolve the maintenance's current notify targets and
	// render the reminder at fire time, so they share the maint store + notifier
	// rather than carrying a pre-rendered payload.
	reg.RegisterProcessor(
		entity.ProcessorTaskMaintReminder,
		reminderprocessor.NewTaskProcessor(stores.Maintenances, services.Notifier),
		messagingProcessorOpts(cfg.Messaging, 0)...,
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
		messagingProcessorOpts(cfg.Messaging, 1)...,
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
	// This is now the only queue-backed delivery path — maintenance notifications
	// are sent inline by the notifier. Invitations keep their own task type rather
	// than a generic one so the registry-routing boundary stays explicit: task
	// types share the goque_task table, and a dedicated type is what binds these
	// messages to the invitation email transport.
	reg.RegisterProcessor(
		entity.ProcessorTaskInvitationEmailSend,
		asyncsenderprocessor.NewTaskProcessor(services.MessageSender),
		messagingProcessorOpts(cfg.Messaging, 0)...,
	)

	// audit.write: domain events published via auditpublisher.Publish land here as
	// rendered audit snapshots (RUK-179). The processor writes audit_log after
	// commit, outside any tx. An idempotent INSERT (ON CONFLICT event_id) makes
	// retries safe.
	reg.RegisterProcessor(
		entity.ProcessorTaskAuditWrite,
		auditprocessor.NewTaskProcessor(services.Audit),
		messagingProcessorOpts(cfg.Messaging, 0)...,
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
		messagingProcessorOpts(cfg.Messaging, 1)...,
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

	// invitation.rotate + invitation.prune: the invitation lifecycle's daily
	// retention pair (flip pending-past-expiry to persisted 'expired', then delete
	// old terminal rows). Extracted to keep this constructor within funlen.
	if err := registerInvitationRotation(reg, cfg, services); err != nil {
		return nil, err
	}

	// license.heartbeat: SaaS mode only — the concrete license service exists
	// exactly when the license is configured, so its presence is the single
	// source of truth for registration (no config/service mismatch possible).
	// The cron job enqueues one empty-payload task per tick (~60s contract
	// cadence; minute-bucketed external id dedupes replicas) and the processor
	// runs one heartbeat: collect report → send → cache. One worker and one
	// attempt — the tick cadence IS the retry, and Console-side failures are
	// already fail-open inside the service (warn + keep cached license).
	licenseEnabled := licenseCfg.Enabled()
	if licenseEnabled {
		reg.RegisterProcessor(
			entity.ProcessorTaskLicenseHeartbeat,
			licenseheartbeatprocessor.NewTaskProcessor(services.licenseHeartbeat),
			goque.WithWorkersCount(1),
			goque.WithTaskProcessingMaxAttempts(1),
		)

		heartbeatJob, err := goque.NewCronJob(
			entity.ProcessorTaskLicenseHeartbeatCron,
			cmp.Or(licenseCfg.CronSpec, "* * * * *"),
			time.UTC,
			licenseheartbeatprocessor.NewTaskFactory(),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to build license-heartbeat cron job: %w", err)
		}
		reg.RegisterPeriodicJob(heartbeatJob)
	}

	if err := reg.verify(entity.ExpectedProcessorTaskTypes(licenseEnabled)); err != nil {
		return nil, err
	}

	return goq, nil
}

// registerInvitationRotation registers the invitation lifecycle's daily
// retention pair on reg:
//
//   - invitation.rotate flips pending invitations past their expiry to the
//     persisted 'expired' status in bounded batches, so the stored status is
//     honest and the email's pending slot is freed.
//   - invitation.prune deletes terminal invitations (by created_at) older than
//     the retention window in bounded batches; pending rows are never pruned.
//
// Both run one worker (a single drained loop that must not run concurrently with
// itself) and use a day-bucketed external id, collapsing multi-replica ticks to
// one enqueue per day — the same shape as audit.prune.
func registerInvitationRotation(reg *processorRegistrar, cfg config.TaskProcessorConfig, services *Services) error {
	rotateCfg := cfg.InvitationRotate
	reg.RegisterProcessor(
		entity.ProcessorTaskInvitationRotate,
		invitationrotateprocessor.NewTaskProcessor(services.Invitation),
		messagingProcessorOpts(cfg.Messaging, 1)...,
	)

	rotateJob, err := goque.NewCronJob(
		entity.ProcessorTaskInvitationRotateCron,
		rotateCfg.CronSpec,
		time.UTC,
		invitationrotateprocessor.NewTaskFactory(rotateCfg.BatchLimit),
	)
	if err != nil {
		return fmt.Errorf("failed to build invitation-rotate cron job: %w", err)
	}
	reg.RegisterPeriodicJob(rotateJob)

	pruneCfg := cfg.InvitationPrune
	reg.RegisterProcessor(
		entity.ProcessorTaskInvitationPrune,
		invitationpruneprocessor.NewTaskProcessor(services.Invitation),
		messagingProcessorOpts(cfg.Messaging, 1)...,
	)

	pruneJob, err := goque.NewCronJob(
		entity.ProcessorTaskInvitationPruneCron,
		pruneCfg.CronSpec,
		time.UTC,
		invitationpruneprocessor.NewTaskFactory(pruneCfg.Retention, pruneCfg.BatchLimit),
	)
	if err != nil {
		return fmt.Errorf("failed to build invitation-prune cron job: %w", err)
	}
	reg.RegisterPeriodicJob(pruneJob)

	return nil
}

// messagingProcessorOpts is the option set every queue-fed processor shares.
//
// FetchTick is applied only when configured: goque's own default (30s) is the
// right answer for a deployed stand, and passing an explicit zero would mean a
// ticker with a zero period rather than "use the default".
//
// workers overrides the configured worker count for the sweep processors that
// must not run concurrently with themselves; pass 0 to take cfg.Workers.
func messagingProcessorOpts(cfg config.TaskProcessorMessagingConfig, workers int) []goque.ProcessorOpts {
	opts := []goque.ProcessorOpts{
		goque.WithWorkersCount(cmp.Or(workers, cfg.Workers)),
		goque.WithTaskProcessingMaxAttempts(cfg.MaxAttempts),
	}
	if cfg.FetchTick > 0 {
		opts = append(opts, goque.WithTaskFetcherTick(cfg.FetchTick))
	}

	return opts
}

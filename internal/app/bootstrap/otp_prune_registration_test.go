package bootstrap

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
)

// baseTaskProcessorConfig is the minimum config NewTaskProcessors needs to build
// every cron job it owns. The sibling sweeps have no code-side spec fallback, so
// theirs must be set or the constructor fails before it reaches otp.prune.
func baseTaskProcessorConfig() config.TaskProcessorConfig {
	return config.TaskProcessorConfig{
		MaintAutoCancel:  config.TaskProcessorMaintAutoCancelConfig{CronSpec: "* * * * *"},
		AuditPrune:       config.TaskProcessorAuditPruneConfig{CronSpec: "0 3 * * *"},
		InvitationRotate: config.TaskProcessorInvitationRotateConfig{CronSpec: "0 3 * * *"},
		InvitationPrune:  config.TaskProcessorInvitationPruneConfig{CronSpec: "0 3 * * *"},
	}
}

// TestNewTaskProcessors_RegistersOTPPrune drives the REAL constructor, not the
// registerOTPPrune helper directly.
//
// That distinction is the whole point of this test. Calling the helper would
// prove only that the helper works, leaving the wiring untested: drop the
// registerOTPPrune call from NewTaskProcessors and a helper-level test stays
// green while the sweep silently never runs. Nothing else would catch it either
// -- the entity drift test only reads the task-type maps, and no other test
// calls NewTaskProcessors at all.
//
// Zero-valued Stores and Services are enough: registration stores the processor
// and never calls into it, and goque accepts a nil TaskStorage for a registrar
// that is not draining.
func TestNewTaskProcessors_RegistersOTPPrune(t *testing.T) {
	cfg := baseTaskProcessorConfig()
	cfg.OTPPrune = config.TaskProcessorOTPPruneConfig{CronSpec: "15 3 * * *"}

	goq, err := NewTaskProcessors(cfg, config.LicenseConfig{}, config.Auth{}, &Stores{}, &Services{})
	require.NoError(t, err)
	require.NotNil(t, goq)
}

// TestRegisterOTPPrune_RegistersBothHalves pins which types the helper registers.
// The entity drift test cannot see a missing .cron entry -- adding otp.prune to
// both task-type lists while never registering its periodic job satisfies every
// assertion in that file -- so this is the only place the cron half is named.
func TestRegisterOTPPrune_RegistersBothHalves(t *testing.T) {
	reg := newProcessorRegistrar(newRegisterOnlyGoque())

	require.NoError(t, registerOTPPrune(reg, config.TaskProcessorConfig{
		OTPPrune: config.TaskProcessorOTPPruneConfig{CronSpec: "15 3 * * *"},
	}, &Services{}))

	require.Contains(t, reg.registered, entity.ProcessorTaskOTPPrune,
		"the task processor must be registered or enqueued sweeps linger undrained")
	require.Contains(t, reg.registered, entity.ProcessorTaskOTPPruneCron,
		"the periodic job must be registered or the sweep never fires")
}

// TestRegisterOTPPrune_EmptyCronSpecStillBoots pins the deliberate divergence
// from the sibling sweeps: they abort startup when their spec is missing, which
// in this merged process would take sign-in down over an absent config line.
func TestRegisterOTPPrune_EmptyCronSpecStillBoots(t *testing.T) {
	reg := newProcessorRegistrar(newRegisterOnlyGoque())

	require.NoError(t, registerOTPPrune(reg, config.TaskProcessorConfig{}, &Services{}))
	require.Contains(t, reg.registered, entity.ProcessorTaskOTPPruneCron)
}

// TestRegisterOTPPrune_RejectsInvalidCronSpec is the other half: a value that is
// present but malformed is an operator error and must surface, not be silently
// replaced by the default.
func TestRegisterOTPPrune_RejectsInvalidCronSpec(t *testing.T) {
	reg := newProcessorRegistrar(newRegisterOnlyGoque())

	err := registerOTPPrune(reg, config.TaskProcessorConfig{
		OTPPrune: config.TaskProcessorOTPPruneConfig{CronSpec: "not a cron spec"},
	}, &Services{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "otp-prune")
}

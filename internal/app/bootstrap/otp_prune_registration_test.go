package bootstrap

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
)

// registerOTPPrune is exercised directly rather than through NewTaskProcessors,
// which needs live stores and services. What matters here is the pair it
// registers: the entity drift test cannot see a missing .cron type -- adding
// otp.prune to both task-type lists while never registering its periodic job
// satisfies every assertion in that file -- so this is the only place the cron
// half is actually pinned.
func TestRegisterOTPPrune_RegistersBothHalves(t *testing.T) {
	reg := newProcessorRegistrar(newRegisterOnlyGoque())

	// A nil Services is enough: registration stores the processor, it does not
	// call the pruner.
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

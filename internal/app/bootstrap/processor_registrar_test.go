package bootstrap

import (
	"testing"

	"github.com/ruko1202/goque"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

// registration touches no storage, so a nil-storage Goque is enough to exercise
// the registrar's coverage check without a DB.
func newRegisterOnlyGoque() *goque.Goque { return goque.NewGoque(nil) }

func TestProcessorRegistrar_VerifyPassesWhenEveryTypeRegistered(t *testing.T) {
	reg := newProcessorRegistrar(newRegisterOnlyGoque())
	for taskType := range entity.ActiveProcessorTaskTypes {
		reg.RegisterProcessor(taskType, goque.NoopTaskProcessor())
	}
	require.NoError(t, reg.verify(entity.ActiveProcessorTaskTypes))
}

func TestProcessorRegistrar_VerifyFailsOnMissingProcessor(t *testing.T) {
	require.Greater(t, len(entity.ActiveProcessorTaskTypes), 1)

	reg := newProcessorRegistrar(newRegisterOnlyGoque())
	// Register every active type except one — that one must be reported missing.
	skipped := true
	for taskType := range entity.ActiveProcessorTaskTypes {
		if skipped {
			skipped = false
			continue
		}
		reg.RegisterProcessor(taskType, goque.NoopTaskProcessor())
	}

	err := reg.verify(entity.ActiveProcessorTaskTypes)
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing")
}

func TestProcessorRegistrar_VerifyFailsOnUnknownType(t *testing.T) {
	// Registering a processor for a task type absent from the expected set
	// is a typo, an undeclared type, or a disabled processor left registered.
	reg := newProcessorRegistrar(newRegisterOnlyGoque())
	for taskType := range entity.ActiveProcessorTaskTypes {
		reg.RegisterProcessor(taskType, goque.NoopTaskProcessor())
	}
	reg.RegisterProcessor("totally.unknown.type", goque.NoopTaskProcessor())

	err := reg.verify(entity.ActiveProcessorTaskTypes)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unexpected")
}

// The license pair is feature-gated: a license-enabled process must
// register both license.heartbeat types, a self-hosted one must register
// neither — ExpectedProcessorTaskTypes carries the toggle into verify.
func TestProcessorRegistrar_VerifyLicenseGated(t *testing.T) {
	registerBaseline := func(reg *processorRegistrar) {
		for taskType := range entity.ActiveProcessorTaskTypes {
			reg.RegisterProcessor(taskType, goque.NoopTaskProcessor())
		}
	}

	t.Run("enabled and registered passes", func(t *testing.T) {
		reg := newProcessorRegistrar(newRegisterOnlyGoque())
		registerBaseline(reg)
		reg.RegisterProcessor(entity.ProcessorTaskLicenseHeartbeat, goque.NoopTaskProcessor())
		reg.RegisterProcessor(entity.ProcessorTaskLicenseHeartbeatCron, goque.NoopTaskProcessor())
		require.NoError(t, reg.verify(entity.ExpectedProcessorTaskTypes(true)))
	})

	t.Run("enabled but not registered fails", func(t *testing.T) {
		reg := newProcessorRegistrar(newRegisterOnlyGoque())
		registerBaseline(reg)
		err := reg.verify(entity.ExpectedProcessorTaskTypes(true))
		require.Error(t, err)
		require.Contains(t, err.Error(), "missing")
	})

	t.Run("disabled but registered fails", func(t *testing.T) {
		reg := newProcessorRegistrar(newRegisterOnlyGoque())
		registerBaseline(reg)
		reg.RegisterProcessor(entity.ProcessorTaskLicenseHeartbeat, goque.NoopTaskProcessor())
		err := reg.verify(entity.ExpectedProcessorTaskTypes(false))
		require.Error(t, err)
		require.Contains(t, err.Error(), "unexpected")
	})

	t.Run("disabled and not registered passes", func(t *testing.T) {
		reg := newProcessorRegistrar(newRegisterOnlyGoque())
		registerBaseline(reg)
		require.NoError(t, reg.verify(entity.ExpectedProcessorTaskTypes(false)))
	})
}

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
	require.NoError(t, reg.verify())
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

	err := reg.verify()
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing")
}

func TestProcessorRegistrar_VerifyFailsOnUnknownType(t *testing.T) {
	// Registering a processor for a task type absent from ActiveProcessorTaskTypes
	// is a typo, an undeclared type, or a disabled processor left registered.
	reg := newProcessorRegistrar(newRegisterOnlyGoque())
	for taskType := range entity.ActiveProcessorTaskTypes {
		reg.RegisterProcessor(taskType, goque.NoopTaskProcessor())
	}
	reg.RegisterProcessor("totally.unknown.type", goque.NoopTaskProcessor())

	err := reg.verify()
	require.Error(t, err)
	require.Contains(t, err.Error(), "unexpected")
}

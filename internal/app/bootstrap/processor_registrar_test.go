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

func TestProcessorRegistrar_VerifyPassesWhenBinaryRegistersExactlyItsTypes(t *testing.T) {
	reg := newProcessorRegistrar(newRegisterOnlyGoque(), entity.ProcessorBinaryAuth)
	for _, taskType := range entity.ProcessorTaskTypesFor(entity.ProcessorBinaryAuth) {
		reg.RegisterProcessor(taskType, goque.NoopTaskProcessor())
	}
	require.NoError(t, reg.verify())
}

func TestProcessorRegistrar_VerifyFailsOnMissingProcessor(t *testing.T) {
	reg := newProcessorRegistrar(newRegisterOnlyGoque(), entity.ProcessorBinaryAuth)
	// Register all but one of the auth-owned types.
	owned := entity.ProcessorTaskTypesFor(entity.ProcessorBinaryAuth)
	require.Greater(t, len(owned), 1)
	for _, taskType := range owned[1:] {
		reg.RegisterProcessor(taskType, goque.NoopTaskProcessor())
	}

	err := reg.verify()
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing")
}

func TestProcessorRegistrar_VerifyFailsOnTypeOwnedByOtherBinary(t *testing.T) {
	// An auth binary that wrongly registers a maintmode-owned type (task-type
	// isolation breach) must fail.
	reg := newProcessorRegistrar(newRegisterOnlyGoque(), entity.ProcessorBinaryAuth)
	for _, taskType := range entity.ProcessorTaskTypesFor(entity.ProcessorBinaryAuth) {
		reg.RegisterProcessor(taskType, goque.NoopTaskProcessor())
	}
	reg.RegisterProcessor(entity.ProcessorTaskMaintReminder, goque.NoopTaskProcessor()) // maintmode-owned

	err := reg.verify()
	require.Error(t, err)
	require.Contains(t, err.Error(), "unexpected")
}

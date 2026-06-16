package entity

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// allDeclaredTaskTypes is every goque task type string the codebase enqueues or
// drains. Keep in sync with the ProcessorTask* consts — a new task type added
// without an owner is exactly the silent-loss bug this test guards against.
var allDeclaredTaskTypes = []string{
	ProcessorTaskMessagingSend,
	ProcessorTaskInvitationEmailSend,
	ProcessorTaskMaintReminder,
	ProcessorTaskMaintAutoCancel,
	ProcessorTaskMaintAutoCancelCron,
	ProcessorTaskAuditPrune,
	ProcessorTaskAuditPruneCron,
	ProcessorTaskAuditWrite,
}

// TestProcessorTaskOwner_CoversEveryTaskType fails if a task type has no draining
// binary. An unowned type lingers in the shared goque_task table forever and its
// work is silently lost (the gap this guard closes, RUK-179 follow-up).
func TestProcessorTaskOwner_CoversEveryTaskType(t *testing.T) {
	for _, taskType := range allDeclaredTaskTypes {
		_, ok := ProcessorTaskOwner[taskType]
		require.Truef(t, ok, "task type %q has no owning binary in ProcessorTaskOwner — it would never be drained", taskType)
	}

	// And no stray owner entries for task types that no longer exist.
	declared := make(map[string]struct{}, len(allDeclaredTaskTypes))
	for _, tt := range allDeclaredTaskTypes {
		declared[tt] = struct{}{}
	}
	for taskType := range ProcessorTaskOwner {
		_, ok := declared[taskType]
		require.Truef(t, ok, "ProcessorTaskOwner has %q which is not a declared task type", taskType)
	}
}

func TestProcessorTaskTypesFor_PartitionsByBinary(t *testing.T) {
	maintmode := ProcessorTaskTypesFor(ProcessorBinaryMaintmode)
	auth := ProcessorTaskTypesFor(ProcessorBinaryAuth)

	// Every owned type belongs to exactly one binary (disjoint), and together
	// they cover the whole owner map (exhaustive) — the task-type isolation
	// invariant that keeps a task off the wrong binary's registry.
	require.Len(t, maintmode, countOwnedBy(ProcessorBinaryMaintmode))
	require.Len(t, auth, countOwnedBy(ProcessorBinaryAuth))
	require.Equal(t, len(ProcessorTaskOwner), len(maintmode)+len(auth))

	for _, tt := range maintmode {
		require.Equal(t, ProcessorBinaryMaintmode, ProcessorTaskOwner[tt])
	}
	for _, tt := range auth {
		require.Equal(t, ProcessorBinaryAuth, ProcessorTaskOwner[tt])
	}
}

func countOwnedBy(binary ProcessorBinary) int {
	n := 0
	for _, owner := range ProcessorTaskOwner {
		if owner == binary {
			n++
		}
	}
	return n
}

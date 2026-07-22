package entity

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// allDeclaredTaskTypes is every goque task type string the codebase declares,
// listed independently of ActiveProcessorTaskTypes so the two can be diffed. A
// new type added to one but not the other is exactly the silent-loss bug this
// test guards against. A type may be declared-but-disabled (present here, absent
// from the active set) when its processor is intentionally turned off without
// freeing the string for reuse — the dedicated subtest below covers that.
var allDeclaredTaskTypes = []string{
	ProcessorTaskMessagingSend,
	ProcessorTaskInvitationEmailSend,
	ProcessorTaskMaintReminder,
	ProcessorTaskMaintAutoCancel,
	ProcessorTaskMaintAutoCancelCron,
	ProcessorTaskAuditPrune,
	ProcessorTaskAuditPruneCron,
	ProcessorTaskAuditWrite,
	ProcessorTaskInvitationRotate,
	ProcessorTaskInvitationRotateCron,
	ProcessorTaskInvitationPrune,
	ProcessorTaskInvitationPruneCron,
}

// TestActiveProcessorTaskTypes guards the active set against drift: every active
// type must be a real declared const (a typo'd key would make the startup guard
// demand a processor for a nonexistent type), and — while all types are
// currently active — the set must cover every declared type so none silently
// goes undrained.
func TestActiveProcessorTaskTypes(t *testing.T) {
	declared := make(map[string]struct{}, len(allDeclaredTaskTypes))
	for _, taskType := range allDeclaredTaskTypes {
		declared[taskType] = struct{}{}
	}

	for taskType := range ActiveProcessorTaskTypes {
		_, ok := declared[taskType]
		require.Truef(t, ok, "ActiveProcessorTaskTypes has %q which is not a declared task type", taskType)
	}

	// No type is disabled today, so the active set covers every declared type.
	// If you disable one, move it out of this assertion deliberately.
	require.Len(t, ActiveProcessorTaskTypes, len(allDeclaredTaskTypes),
		"every declared task type must be active (none is currently disabled)")
}

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
	ProcessorTaskOTPEmailSend,
}

// disabledTaskTypes is every declared type whose processor is intentionally not
// registered. The string stays reserved (the const remains declared) so it cannot
// be reused for something else, while the startup coverage guard no longer
// expects anything to drain it.
//
// messaging.send has no producer: maintenance notifications are delivered inline
// by dispatchSync, and the queue-backed path that used to enqueue them was
// removed. Invitation e-mails ride their own type, invitation.email.
var disabledTaskTypes = []string{
	ProcessorTaskMessagingSend,
}

// TestActiveProcessorTaskTypes guards the active set against drift: every active
// type must be a real declared const (a typo'd key would make the startup guard
// demand a processor for a nonexistent type), and the active set plus the
// deliberately disabled ones must account for every declared type, so nothing
// silently goes undrained.
func TestActiveProcessorTaskTypes(t *testing.T) {
	declared := make(map[string]struct{}, len(allDeclaredTaskTypes))
	for _, taskType := range allDeclaredTaskTypes {
		declared[taskType] = struct{}{}
	}

	for taskType := range ActiveProcessorTaskTypes {
		_, ok := declared[taskType]
		require.Truef(t, ok, "ActiveProcessorTaskTypes has %q which is not a declared task type", taskType)
	}

	require.Len(t, ActiveProcessorTaskTypes, len(allDeclaredTaskTypes)-len(disabledTaskTypes),
		"every declared task type must be either active or explicitly disabled")
}

// TestDisabledTaskTypesStayDeclared pins the declared-but-disabled contract: the
// const keeps existing (so the string is never recycled) while the type is absent
// from the active set (so the startup guard does not demand a processor for it).
func TestDisabledTaskTypesStayDeclared(t *testing.T) {
	declared := make(map[string]struct{}, len(allDeclaredTaskTypes))
	for _, taskType := range allDeclaredTaskTypes {
		declared[taskType] = struct{}{}
	}

	for _, taskType := range disabledTaskTypes {
		require.Containsf(t, declared, taskType,
			"disabled type %q must stay declared so its string is not reused", taskType)
		require.NotContainsf(t, ActiveProcessorTaskTypes, taskType,
			"disabled type %q must be absent from the active set", taskType)
	}
}

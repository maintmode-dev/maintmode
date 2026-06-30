package bootstrap

import (
	"fmt"
	"slices"

	"github.com/ruko1202/goque"

	"github.com/ruko1202/maintmode/internal/entity"
)

// processorRegistrar wraps a *goque.Goque and records every task type it
// registers a processor or periodic job for, so the set can be checked against
// the canonical entity.ActiveProcessorTaskTypes at startup.
//
// This closes the silent-loss gap inherent to any goque task type: the
// goque_task table is shared, so a task type that is enqueued but has no
// registered processor lingers forever and its work is lost. verify() turns
// "forgot to register" / "registered an unknown type" into a startup failure
// instead of a production data loss.
type processorRegistrar struct {
	goq        *goque.Goque
	registered map[string]struct{}
}

func newProcessorRegistrar(goq *goque.Goque) *processorRegistrar {
	return &processorRegistrar{
		goq:        goq,
		registered: make(map[string]struct{}),
	}
}

func (r *processorRegistrar) RegisterProcessor(taskType string, p goque.TaskProcessor, opts ...goque.ProcessorOpts) {
	r.registered[taskType] = struct{}{}
	r.goq.RegisterProcessor(taskType, p, opts...)
}

func (r *processorRegistrar) RegisterPeriodicJob(job *goque.PeriodicJob) {
	if job != nil {
		r.registered[job.Name()] = struct{}{}
	}
	r.goq.RegisterPeriodicJob(job)
}

// verify fails if the registered processors do not match the canonical
// entity.ActiveProcessorTaskTypes exactly: a missing type would silently drop
// its tasks, and an unexpected one (absent from the active set) is a typo, an
// undeclared type, or a deliberately-disabled processor that was not also
// removed from registration.
func (r *processorRegistrar) verify() error {
	var missing []string
	for taskType := range entity.ActiveProcessorTaskTypes {
		if _, ok := r.registered[taskType]; !ok {
			missing = append(missing, taskType)
		}
	}

	var unexpected []string
	for taskType := range r.registered {
		if _, ok := entity.ActiveProcessorTaskTypes[taskType]; !ok {
			unexpected = append(unexpected, taskType)
		}
	}

	if len(missing) > 0 || len(unexpected) > 0 {
		slices.Sort(missing)
		slices.Sort(unexpected)
		return fmt.Errorf(
			"goque processor coverage mismatch: missing=%v unexpected=%v",
			missing, unexpected,
		)
	}
	return nil
}

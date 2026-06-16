package bootstrap

import (
	"fmt"
	"slices"

	"github.com/ruko1202/goque"

	"github.com/ruko1202/maintmode/internal/entity"
)

// processorRegistrar wraps a *goque.Goque and records every task type it
// registers a processor or periodic job for, so the set can be checked against
// the canonical entity.ProcessorTaskOwner map at startup.
//
// This closes the silent-loss gap that is inherent to any goque task type (not
// just audit): the goque_task table is shared, so a task type that is enqueued
// but has no registered processor on any binary lingers forever and its work is
// lost. verify() turns "forgot to register" / "registered on the wrong binary"
// into a startup failure instead of a production data loss.
type processorRegistrar struct {
	goq        *goque.Goque
	binary     entity.ProcessorBinary
	registered map[string]struct{}
}

func newProcessorRegistrar(goq *goque.Goque, binary entity.ProcessorBinary) *processorRegistrar {
	return &processorRegistrar{
		goq:        goq,
		binary:     binary,
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

// verify fails if the binary did not register exactly the task types it owns per
// entity.ProcessorTaskOwner: a missing one would silently drop tasks; an extra
// one means it drains a type another binary owns (task-type isolation breach).
func (r *processorRegistrar) verify() error {
	want := entity.ProcessorTaskTypesFor(r.binary)

	var missing []string
	for _, taskType := range want {
		if _, ok := r.registered[taskType]; !ok {
			missing = append(missing, taskType)
		}
	}

	var unexpected []string
	for taskType := range r.registered {
		if entity.ProcessorTaskOwner[taskType] != r.binary {
			unexpected = append(unexpected, taskType)
		}
	}

	if len(missing) > 0 || len(unexpected) > 0 {
		slices.Sort(missing)
		slices.Sort(unexpected)
		return fmt.Errorf(
			"goque processor coverage mismatch for binary %q: missing=%v unexpected=%v",
			r.binary, missing, unexpected,
		)
	}
	return nil
}

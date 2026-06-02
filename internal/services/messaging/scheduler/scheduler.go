// Package scheduler schedules and cancels delayed goque tasks of any registered
// type. It is the queue-facing counterpart of messagesender: where messagesender
// delivers a pre-rendered message, scheduler enqueues a domain task (e.g.
// maint.reminder) whose processor resolves recipients and renders content at
// processing time. Both share the same goque queue but keep distinct
// responsibilities.
package scheduler

import (
	"github.com/ruko1202/goque"
)

// Service schedules delayed tasks on the goque queue.
type Service struct {
	queue goque.TaskQueueManager
}

func NewService(queue goque.TaskQueueManager) *Service {
	return &Service{queue: queue}
}

// Package auditprocessor drains audit.write goque tasks: it decodes the
// rendered audit snapshot and writes it to audit_log (RUK-179). It is the
// durable, after-commit replacement for the former in-process auditor listener,
// and it runs outside any transaction so an audit write — or any future
// network-backed listener that joins this pattern — can never hold a business
// tx (closes review finding H3 by construction).
package auditprocessor

import (
	"context"

	"github.com/ruko1202/goque"

	"github.com/ruko1202/maintmode/internal/entity"
)

// AuditWriter persists a rendered audit entry. Defined consumer-side so the
// processor can be unit-tested with a fake.
type AuditWriter interface {
	AddLog(ctx context.Context, entry *entity.AuditEntry) error
}

type processor struct {
	auditor AuditWriter
}

func newQueueProcessor(auditor AuditWriter) *processor {
	return &processor{auditor: auditor}
}

// NewTaskProcessor returns the goque TaskProcessor for ProcessorTaskAuditWrite.
// A payload that fails to decode is canceled rather than retried (goque exports
// goque_payload_decode_errors_total for visibility — alerted on in
// monitoring/config/alerts.yml).
func NewTaskProcessor(auditor AuditWriter) goque.TaskProcessor {
	return goque.NewTypedTaskProcessor[entity.ProcessorTaskPayloadAuditWrite](
		newQueueProcessor(auditor),
		goque.WithCancelTaskWhenPayloadDecodeError[entity.ProcessorTaskPayloadAuditWrite](),
	)
}

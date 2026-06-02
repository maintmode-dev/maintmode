---
name: goque-async-patterns
description: Async task queue patterns for MaintMode using goque — scheduling delayed tasks, the transactional outbox, idempotency, typed processors, and the sender vs scheduler split. Use when enqueuing work, scheduling delayed/deferred tasks, writing or registering a goque processor, or wiring anything that writes to the DB and the queue together.
---

# goque Async Patterns (MaintMode)

The async task queue is goque. Tasks have a string **type**, a JSON **payload**,
and an **external_id** used for idempotency. Producers schedule tasks; typed
processors consume them. Anchors:

- Scheduling: `internal/services/messaging/scheduler` (`ScheduleDelayed`, `Cancel`)
- Delivery facade: `internal/services/messaging/sender`
- Processors: `internal/services/messaging/{taskprocessor,reminderprocessor}`
- Registration: `internal/app/bootstrap/processors.go`
- Tx bridge: `internal/utils/dbtx`, `goque.WithTx`

## Rule 1 — Enqueue inside the triggering transaction (outbox)

A DB state change and the task it triggers must be atomic. Enqueue **inside** the
business transaction and let an enqueue error roll it back. Never commit the
write and enqueue afterwards — a crash in between loses or orphans the task.

The scheduler bridges the maintmode tx into goque:

```go
if tx, ok := dbtx.TxFromContext(ctx); ok {
    ctx = goque.WithTx(ctx, tx) // insert/cancel joins the caller's tx
}
```

Schedule through `messaging/scheduler`, not goque directly. Example: reminders
are enqueued inside the approve transaction so the status change and the queued
tasks commit together (`internal/services/deferrednotifications/enqueue.go`,
`internal/services/maint/appove_maint.go`).

## Rule 2 — Payload is identifiers, not rendered content

One domain task per unit of work. The payload carries ids
(`{maint_id, deferred_id}`); the processor loads the entity and resolves
recipients / renders text at fire time. Do not fan out into N pre-rendered
tasks and do not snapshot data that can be read when the task runs — this keeps
the task consistent with the latest state and avoids stale payloads.

## Rule 3 — Idempotency via external_id

Derive `external_id` from a stable key with `internal/utils/xhash`.`HashSha256`,
e.g. `maint|reminder|<maintID>|<deferredID>`. goque's unique `(type,
external_id)` index then collapses duplicate enqueues. Do not hand-roll
sha256/hex.

## Rule 4 — Processor shape

```go
func New(loader Loader, notifier Notifier) goque.TaskProcessor {
    return goque.NewTypedTaskProcessor[entity.PayloadType](
        &processor{...},
        goque.WithCancelTaskWhenPayloadDecodeError[entity.PayloadType](),
    )
}

func (p *processor) ProcessTask(ctx context.Context, task *goque.TypedTask[entity.PayloadType]) error {
    obj, err := p.loader.Get(ctx, task.Payload.ID)
    if errors.Is(err, apperr.ErrNotFound) {
        return nil // gone: skip, do not retry
    }
    if err != nil {
        return err // transient: let goque retry
    }
    if !stillMeaningful(obj.Status) {
        return nil // re-check domain state; skip if no longer relevant
    }
    return p.do(ctx, obj)
}
```

- Depend on domain via narrow local interfaces (`Loader`, `Notifier`), not
  concrete services.
- Return `nil` to skip (no retry); return an error only for transient failures
  worth retrying.
- A scheduled task can fire after the entity changed/was canceled — the status
  re-check is the primary guard, separate from best-effort cancellation.

## Rule 5 — sender vs scheduler

- `messaging/sender` delivers an already-rendered message (`Send`/`SendAsync`).
- `messaging/scheduler` enqueues a delayed domain task whose processor resolves
  content; it owns all goque enqueue/cancel plumbing.
- `sender` builds on `scheduler`; do not widen `sender` into a generic scheduler
  and do not call goque from both.

## Cancellation

Cancel pending tasks by stored task id through `scheduler.Cancel` (tx-aware).
Treat cancellation as best-effort cleanup — the processor's status guard
(Rule 4) is what actually prevents acting on stale work.

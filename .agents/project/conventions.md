# Code Conventions

## Principles

- Prefer simple, explicit code.
- Follow existing project patterns before adding new abstractions.
- Keep domain logic out of HTTP handlers and storage mappers.
- Keep unrelated refactors out of feature and bug-fix changes.
- Add comments only for non-obvious decisions.

## Go Style

- Use `gofmt` and goimports through `make fmt`.
- Keep packages short, lowercase, and singular when possible.
- Use underscores for multi-word file names.
- Export only what is needed outside the package.
- Put `context.Context` first in functions that do I/O or may block.

## Naming

Packages:

```go
package maint
package calendar
package dbtx
```

Files:

```text
create_draft.go
update_maint.go
step_lifecycle.go
```

Functions and types:

```go
type Service struct {}
func NewService(...) *Service
func (s *Service) UpdateMaint(ctx context.Context, cmd *entity.UpdateMaintenanceCmd) error
```

## Layering

API packages:

- Bind and validate requests.
- Convert API models to entity commands.
- Convert service responses to API responses.
- Map domain errors to HTTP errors.

Service packages:

- Own domain workflows and status transitions.
- Validate commands.
- Coordinate stores and transactions.
- Return domain errors from `internal/apperr`.

Storage packages:

- Own SQL and Jet statements.
- Map DB models to entities.
- Translate storage-specific not-found errors.
- Avoid domain workflow decisions.

Entity packages:

- Own domain types, commands, statuses, and transition helpers.
- Avoid infrastructure dependencies.

## Error Handling

- Return errors; do not panic in normal request paths.
- Wrap lower-level errors with useful context when it helps debugging.
- Preserve sentinel/domain errors so callers can use `errors.Is`.
- Map API errors in the API layer, not in services.

Good:

```go
if err := s.store.UpdateStep(ctx, maintID, step); err != nil {
    return fmt.Errorf("update maintenance step: %w", err)
}
```

## Transactions

- Use the project transaction manager for multi-write workflows.
- Keep read/modify/write state transitions inside one transaction.
- Lock rows explicitly when concurrent updates can race.
- Keep transaction callbacks focused; do not hide unrelated I/O inside them.

## Testing

- Name test files after the file under test: `update_maint_test.go`.
- Name tests after behavior: `TestUpdateDraft`.
- Use table tests when cases share setup and assertions.
- Add regression tests for bug fixes.
- Prefer `require` for setup failures and direct invariants.
- Use `make tloc` for the default internal package test suite.

## Generated Code

- Do not manually edit generated files unless the generator output itself is
  the artifact under review.
- Regenerate from the source schema/spec and commit source plus generated
  changes together.

## Comments

Use comments for why, not what.

Good:

```go
// Lock steps with the maintenance row so lifecycle transitions see one order.
```

Avoid:

```go
// Set status to completed.
step.Status = entity.MaintenanceStepStatusCompleted
```

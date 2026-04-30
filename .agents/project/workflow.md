# Development Workflow

## Branch Safety

Never commit directly to `main` or `master`.

Before making commit-ready changes:

```bash
git branch --show-current
```

If the current branch is `main` or `master`, create a topic branch first. Branch
names must follow [branch-naming.md](./branch-naming.md).

## Typical Change Flow

1. Inspect current worktree state.
2. Understand the relevant code and tests before editing.
3. Make focused changes.
4. Format changed Go files.
5. Run the narrowest useful tests.
6. Run the broader project target before handing off.
7. Commit only when the user asks, using [commits.md](./commits.md).

Useful commands:

```bash
git status --short
make tloc
make tloc-api
make tloc-all
make lint
make fmt
```

`make tloc` is the default local validation target for internal Go packages.
It sets the local MaintMode config and secrets paths from the Makefile.

## Feature Work

- Start from the domain model and service boundary.
- Add or update tests near the behavior being changed.
- Keep HTTP binding, service logic, and storage changes in their own layers.
- Add migrations when persistence shape changes.
- Regenerate generated code only when the source schema/spec changed.

## Bug Fixes

- Reproduce the bug with a focused test when practical.
- Fix the smallest behavior surface that explains the bug.
- Keep the regression test after the fix.
- Check nearby edge cases and transaction boundaries.

## API Changes

- Keep request binding and validation in API packages.
- Map API models to entity commands before entering services.
- Keep service errors domain-level; map them to HTTP in API error handling.
- Update Swagger/generated clients when public contracts change.
- Run API tests with `make tloc-api` when endpoint behavior changes.

## Storage Changes

- Use existing Jet/sqlx storage patterns.
- Translate not-found database errors into domain errors at storage boundaries.
- Use transactions for multi-table mutations.
- Add migrations for schema changes.
- Verify lock behavior for concurrent state changes.

## Review Checklist

Before asking for review or handing work back:

- Tests relevant to the touched code pass.
- `make tloc` passes unless blocked by environment.
- Code is formatted.
- Error paths preserve useful context.
- Transactions cover all related writes.
- Public API docs/specs match handlers and models.
- Generated files are updated only when their source changed.
- Unrelated dirty worktree changes are left untouched.

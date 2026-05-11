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
make docker-up
make tloc
make app-up args="--build"
make tloc-api
make tloc-all
make lint
make fmt
```

Backend tests are split into deterministic gates:

- CI runs lint through the official `golangci/golangci-lint-action` with the
  repository `.golangci.yaml` configuration.
- CI builds both service binaries with the official `goreleaser/goreleaser-action`
  for `maintmode` and `auth`.
- CI backend tests run `make docker-up` first, then `make tloc`.
- CI API e2e tests load the images built by the image stage, run `make app-up`,
  then `make tloc-api`. The API test suite waits for
  `http://localhost:9001/maintmode/readiness` before executing test cases.
- CI builds Docker images for `maintmode` and `auth`. Pull requests only build
  images; pushes to `main`, `master`, or `v*` tags publish to GHCR under
  `ghcr.io/<owner>/<repo>/<service>`.
- `make tloc` runs unit and DB-backed internal Go tests together with local
  config/secrets paths and no extra build tags.
- `make tloc-api` runs API e2e tests against an already available API with the
  `api` build tag.
- `make tloc-all` runs `make tloc` and `make tloc-api` in sequence.

API e2e tests must use the `api` build tag. Internal backend tests must not use
an additional build tag.

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

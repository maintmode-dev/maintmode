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

The project ships as a **single binary**: `cmd/maintmode`. There is no separate
`auth` (or notificator) process — since `c650ffc4` the codebase is a modular
monolith. Module boundaries (core / auth / notificator / audit / integration /
license) are logical, enforced at lint time by the `depguard` rules in
`.golangci.yaml` (the store-fortress rules, commented there as
"Store fortress"), not by separate processes. A
store belongs to exactly one module and may only be imported from that module;
cross-module data access goes through a consumer-declared interface. Because
everything runs in one process, there is no version skew *between* modules —
do not plan around one binary knowing about a column another does not.

Backend tests are split into deterministic gates (`.github/workflows/backend-ci.yml`):

- CI runs lint through the official `golangci/golangci-lint-action` with the
  repository `.golangci.yaml` configuration and `--build-tags=api`, so API e2e
  test files are linted too.
- CI runs `govulncheck` via `make vuln`.
- CI builds the single `maintmode` binary with the official
  `goreleaser/goreleaser-action` from
  `deployment/maintmode/.build/.goreleaser.yaml`. This is the only place the
  binary is built; the image job reuses the uploaded artifact instead of
  rebuilding it in Docker.
- CI backend tests run `make docker-up` first, then `make tloc-cov` — not
  `make tloc`. `tloc-cov` is the only target running with `-race`, so the
  concurrency tests actually exercise the race detector in CI.
- CI builds Docker images for `maintmode` and `migrations` and publishes them
  to `ghcr.io/<owner>/<repo>/maintmode` and
  `ghcr.io/<owner>/<repo>/migrations`.
- Image builds and API e2e tests run **only** on pushes to `main`/`master` and
  `v*` tags. Pull requests run just lint, vuln, binary build, and backend tests.
- CI API e2e tests pull the published `sha-`tagged image, retag it to the local
  CI tag, run `make app-up`, then `make tloc-api`. The API test suite waits for
  `http://localhost:9001/maintmode/readiness` before executing test cases.
- `make tloc` runs unit and DB-backed internal Go tests together with local
  config/secrets paths and no extra build tags. It is the default *local*
  validation target; CI uses `tloc-cov` instead.
- `make tloc-api` runs API e2e tests against an already available API with the
  `api` build tag.
- `make tloc-all` runs `make tloc` and `make tloc-api` in sequence.

API e2e tests must use the `api` build tag. Internal backend tests must not use
an additional build tag.

## Design Gates

Run these before writing code, not after review finds the gap:

1. **Minimal schema.** What is the smallest set of tables/columns? Can an
   existing table be reused instead of a new child table? Store only what cannot
   be derived at read time; resolve the rest (recipients, rendered text) when the
   data is read or the task fires.
2. **Atomicity.** Does the operation touch both the database and a queue/external
   side effect? If so it must be one transaction via the outbox (see the queue
   rules in conventions.md). No "commit then enqueue".
3. **Reuse.** Before a new abstraction/utility/option type, grep `internal/utils`
   and sibling services; a new abstraction needs a reason why the existing one
   does not fit.
4. **Product semantics.** For every trigger/notification, list the statuses in
   which it is meaningful and guard for them at execution time.

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
- Transactions cover all related writes, and queue enqueues use the outbox.
- Public API docs/specs match handlers and models.
- Generated files are updated only when their source changed.
- Unrelated dirty worktree changes are left untouched.
- No dead code (unused options, unwired branches) is left behind.
- Self-review the diff before handing back: run the `code-reviewer` and
  `code-skeptic` skills on your own change and resolve their findings, so human
  review starts from a clean baseline rather than catching basics.

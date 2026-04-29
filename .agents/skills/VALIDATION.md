# Skills Scenario Validation

Date: 2026-04-29
Branch: `codex/skills-migration`

## Method

Each remaining skill was checked with a realistic MaintMode prompt:

> Use `$skill-name` at `.agents/skills/<skill-name>` to solve a MaintMode task in the matching layer.

The check asked whether the skill adds repository-specific value beyond general Go/Postgres/Echo knowledge: current package names, real helpers, MaintMode paths, generated code conventions, Makefile workflows, and reference quality.

## Final Keep Set

| Skill | Helps? | Best Fit | Scenario Result |
|---|---:|---|---|
| `transaction-patterns` | Yes | Multi-store service mutations using `internal/utils/dbtx.TxManager`. | Useful for `WithinTx`, context-carried transactions, `dbtx.Executor`, short transaction boundaries, and `FOR UPDATE` flows already used by `maint`, `auth`, `user`, and `resources` services. |
| `jet-sqlx` | Yes | PostgreSQL stores in `internal/storages/*`. | Useful for Jet v2 generated `table/model`, `qrm.ErrNoRows` translation, entity/model mappers, `make db-models`, and `s.db.Executor(ctx)`. |
| `error-handling-go` | Yes | Error propagation across `apperr`, services, stores, and API handlers. | Useful after tightening to real MaintMode anchors: `internal/apperr`, `apierrors.ToAPIErrResponse`, `apierrors.ValidationErr`, Jet/qrm store translation, and xlog-safe logging. |
| `echo-v4-framework` | Yes | Echo v4 handlers, middleware, route wiring, and API tests. | Useful as an Echo v4 guardrail and handler checklist for `internal/app/api/*`, `internal/server`, `echo.Context`, request context propagation, and `apierrors.ToAPIErrResponse`. |
| `zap-logging` | Yes | xlog spans/logs in services, stores, handlers, and startup. | Useful because MaintMode uses `github.com/ruko1202/xlog` and `xfield`, not raw zap in application code. |
| `supabase-postgres-best-practices` | Yes | SQL/index/schema performance review. | Useful as a marketplace reference pack for EXPLAIN, indexes, connection/pooling rules, locking, pagination, and query review. Less MaintMode-specific, but high-value for Postgres decisions. |

## Deleted Skills

These were removed because scenario testing showed they were mostly tutorials, generic patterns, or Makefile duplication:

| Skill | Reason |
|---|---|
| `golang-coder` | Too close to general Go conventions and local `.agent/*.md` docs. |
| `testify-testing` | Mostly a testing guide; MaintMode-specific test patterns are better found in nearby tests and generated mocks. |
| `goose-migrations` | Mostly duplicates Makefile targets and standard goose workflow. |
| `prometheus-metrics` | Too generic until MaintMode has stronger custom metrics conventions. Existing OTel/Prometheus wiring should be read directly. |

## Final Checks

- All kept skills include `agents/openai.yaml`.
- Removed skills no longer exist under `.agents/skills`.
- `quick_validate.py` passes for all kept skills.
- Spot checks found no stale implementation references to removed skills or previously rejected API/database/testing/logging patterns.

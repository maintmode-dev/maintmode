# MaintMode Agent Guide

This file is the root entrypoint for AI agents working in this repository.
Detailed project notes live in `.agents/project/`.

## Read First

Before making changes, read the focused file for the task:

- `.agents/project/project.md` - product context, architecture, stack
- `.agents/project/workflow.md` - workflow and validation commands
- `.agents/project/conventions.md` - Go and project coding conventions
- `.agents/project/branch-naming.md` - branch naming rules
- `.agents/project/commits.md` - commit message rules

## Repository Scope

This repository is the MaintMode backend. It is a Go service for planning,
reviewing, and executing maintenance windows while detecting time/resource
conflicts.

Keep code organized by layer:

- API packages bind requests, validate input, shape responses, and map errors.
- Service packages own domain workflows, status transitions, and transactions.
- Storage packages own SQL, Jet statements, and database model mapping.
- Entity packages own domain types, commands, statuses, and transition helpers.

## Required Workflow

- Check the current worktree before editing: `git status --short`.
- Do not overwrite unrelated user changes.
- Never commit directly to `main` or `master`.
- Commit only when the user explicitly asks.
- Use `make tloc` as the default local validation target.
- Use `make tloc-api` when API integration behavior changes.
- Use `make fmt` for Go formatting and import organization.
- Use `make lint` when code quality checks are requested or before handoff.

## Implementation Rules

- Prefer existing project patterns over new abstractions.
- Keep changes focused on the requested behavior.
- Add or update tests near the changed behavior.
- Use transactions for multi-table or read-modify-write state changes.
- Preserve domain errors so callers can use `errors.Is`.
- Update Swagger/generated clients only when public API contracts change.
- Do not manually edit generated files unless generator output is the artifact
  under review.

## Validation

Default check:

```bash
make tloc
```

Broader checks:

```bash
make tloc-api
make tloc-all
make lint
```

If a check cannot run because of local infrastructure, report the exact blocker
and any narrower checks that did run.

## Documentation Maintenance

When project rules change, update the focused file in `.agents/project/`.
Do not duplicate complete rules across multiple files; link to the owner file
instead.

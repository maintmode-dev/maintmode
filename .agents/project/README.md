# Agent Project Notes

This directory contains project-level context for agents and developers working
on MaintMode. Keep each rule in one place and link to it from other files.

## Files

- [project.md](./project.md) - product context, architecture, stack
- [workflow.md](./workflow.md) - day-to-day development workflow and checks
- [branch-naming.md](./branch-naming.md) - branch naming rules
- [commits.md](./commits.md) - commit message rules
- [conventions.md](./conventions.md) - Go and project coding conventions

## Reading Order

1. Read [project.md](./project.md) for domain and architecture context.
2. Read [workflow.md](./workflow.md) before changing code.
3. Use [branch-naming.md](./branch-naming.md) only when creating a branch.
4. Use [commits.md](./commits.md) only when preparing commits.
5. Use [conventions.md](./conventions.md) while implementing or reviewing code.

## Maintenance Rules

- Do not duplicate full rules across files. Add a short pointer instead.
- Update the focused file that owns the topic.
- Keep examples project-specific.
- Prefer current Makefile targets over stale command snippets.

## Ownership

- Project/product/stack changes: `project.md`
- Git flow and validation commands: `workflow.md`
- Branch name policy: `branch-naming.md`
- Commit message policy: `commits.md`
- Go style, tests, errors, storage/API patterns: `conventions.md`

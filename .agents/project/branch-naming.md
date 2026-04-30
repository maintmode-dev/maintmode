# Branch Naming

Use this file only for branch names. General git workflow lives in
[workflow.md](./workflow.md).

## Format

```
<type>/<word1>-<word2>-<word3>
```

Rules:

- Use a known type prefix.
- Use lowercase only.
- Use hyphens between words.
- Use at most 3 words after the prefix.
- Do not include issue numbers unless the user explicitly asks.
- Keep the name descriptive, not generic.

## Types

| Type | Use for | Example |
| --- | --- | --- |
| `feature/` | New features or enhancements | `feature/add-steps` |
| `fix/` | Bug fixes | `fix/step-update` |
| `docs/` | Documentation-only changes | `docs/project-notes` |
| `refactor/` | Restructuring without behavior changes | `refactor/maint-store` |
| `test/` | Test-only changes | `test/step-lifecycle` |
| `chore/` | Maintenance and dependencies | `chore/update-deps` |
| `perf/` | Performance improvements | `perf/conflict-query` |
| `style/` | Formatting-only changes | `style/goimports` |
| `ci/` | CI configuration | `ci/add-checks` |
| `build/` | Build system changes | `build/update-makefile` |
| `revert/` | Reverts | `revert/bad-migration` |

## Examples

Good:

```bash
feature/add-steps
fix/step-update
docs/project-notes
refactor/maint-store
test/step-lifecycle
```

Bad:

```bash
feature/add-new-maintenance-step-lifecycle-system
feature/addSteps
feature/add_steps
fix/bug
feature/issue-123-add-steps
```

## Shortening

- Remove redundant words: `feature/add-new-webhook-system` -> `feature/webhook-system`
- Use common abbreviations: `authentication` -> `auth`
- Keep the domain noun: `fix/resolve-race-condition-in-worker-pool` -> `fix/worker-race`

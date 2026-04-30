# Commit Messages

Use Conventional Commits.

## Format

```
<type>(<scope>): <subject>

<body>

<footer>
```

Only the first line is required.

## Types

- `feat` - new feature
- `fix` - bug fix
- `docs` - documentation only
- `style` - formatting only
- `refactor` - code restructuring without behavior changes
- `perf` - performance improvement
- `test` - tests
- `chore` - maintenance, dependencies, generated updates
- `ci` - CI configuration
- `build` - build system
- `revert` - revert a previous commit

## Scope

Scope is optional. Use a short affected area when it adds clarity:

- `api`
- `auth`
- `calendar`
- `db`
- `maint`
- `migration`
- `storage`
- `test`
- `docs`

## Subject

- Use imperative present tense: `add`, not `added` or `adds`.
- Start lowercase.
- Do not end with a period.
- Keep it short, ideally 50 characters or less.

## Body

Use a body when the change is not obvious from the subject. Explain why the
change exists and any important implementation details.

Keep each line around 72 characters.

## Footer

Use the footer for:

- `BREAKING CHANGE: ...`
- `Closes #123`
- `Fixes #456`
- `Co-authored-by: Name <email>`

## Examples

```text
feat(maint): add step lifecycle

Add start, complete, and cancel transitions for maintenance steps.
Block maintenance completion until every step reaches a terminal state.
```

```text
fix(storage): preserve resources on draft update

Only replace maintenance resources when the update command supplies a
resource list. Metadata-only updates must not delete child rows.

Fixes #123
```

```text
docs: clean project agent notes
```

## Commit Shape

- One logical change per commit.
- Each commit should compile and pass relevant tests.
- Keep documentation-only changes separate when practical.
- Squash fixup commits before publishing or merging.

Do not use vague messages such as `update`, `fix bug`, or `wip`.

# Contributing

Thanks for taking an interest. This document covers how to get the backend
running locally and what a change is expected to look like before it is merged.

## Before you start on something large

Open an issue first and describe what you want to change. This project has a
fairly opinionated architecture, and a large pull request that cuts across it is
much harder to accept than to discuss beforehand. Small fixes — a bug, a typo, a
missing test — need no preamble; just send them.

## Getting set up

Requirements: Go (see the `go` directive in `go.mod`), Docker with Compose, and
`make`.

```bash
make deps          # tool dependencies into ./bin
make secrets       # create deployment/maintmode/<env>/app.secrets.yaml from the samples
make docker-up     # postgres, pg_doorman, valkey
make db-up         # apply migrations
make run           # start the service
```

`make secrets` only creates files that do not exist yet, so it will not
overwrite local edits. The generated `app.secrets.yaml` files are gitignored —
that is deliberate, and adding one to a commit is the single easiest way to leak
a credential. Please check `git status` before you commit.

## Checks

```bash
make fmt           # gofmt + goimports
make lint          # golangci-lint
make tloc          # unit tests
make test-api      # API integration tests (brings up a compose stack)
```

CI runs lint, a vulnerability check, unit tests, API tests, and builds the
images. A pull request is expected to be green before review.

## What a change should include

- **Tests.** New behaviour comes with tests; a bug fix comes with a test that
  fails before the fix.
- **Generated code stays generated.** If you touch an interface with a mock, or
  a query, re-run `make mocks` / `make db-models` rather than editing the
  generated file. Swagger annotations: `make swag`.
- **Comments explain why.** The codebase leans on comments that record the
  reasoning behind a decision — why this approach and not the obvious one.
  Please match that. A comment restating what the line already says is worse
  than none.
- **Migrations are additive.** `make db-migrate-create name=<name>`. Assume an
  older version of the service is still running against the database during a
  deploy.

## Commits and pull requests

Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/)
(`feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `chore:`). Write the body to
explain the reasoning, not the diff — the diff is already there.

You may see `RUK-123` identifiers in the history and in comments. Those are
issue references from the project's own tracker, which is not public. You are
not expected to add them.

## Licence

By contributing you agree that your contribution is licensed under the
[GNU AGPL v3](LICENSE), the same licence as the project.

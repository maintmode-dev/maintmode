# MaintMode

**Maintenance calendar for engineering teams.** Plan technical work without
collisions and without surprise incidents.

Several teams share the same infrastructure. Two changes touch the same database
in overlapping windows and nobody notices until something breaks. MaintMode makes
that visible before it happens:

> **Time + shared resources = risk.**

The service keeps one calendar of maintenance work, tracks which resources each
piece of work touches, separates what was *planned* from what actually *happened*,
and flags overlaps as conflicts.

This repository is the backend — a Go service exposing the API. The web interface
lives in [maintmode-ui](https://github.com/maintmode-dev/maintmode-ui).

## Status

Used in production by its author, and open to anyone who wants to run it. The
API is not versioned for external consumers yet, so expect breaking changes
between releases.

## Self-hosting

Self-hosting is free and unlimited — no seat counting, no licence checks, no
phoning home. See [Licensing and telemetry](#licensing-and-telemetry) below for
exactly what that means in the code.

The quickest way to get an instance running is
[maintmode-selfhost](https://github.com/maintmode-dev/maintmode-selfhost), which
provides a Docker Compose setup and step-by-step instructions. What follows here
is for working on the backend itself.

## Features

- One calendar of maintenance work — week and month views
- Planned window vs. actual execution time, tracked separately
- Shared resources: services, databases, clusters
- Conflict detection on overlapping work against shared resources
- Approval flows for work that needs sign-off
- Notifications to Slack, Telegram, and email
- Role-based access control, invitation-based user management
- Audit log

## Architecture

One Go binary serves both the authentication routes (`/auth/*`) and the
application API (`/maintmode/*`) — there is no separate auth service.

- **Language**: Go (see `go.mod` for the required version)
- **HTTP**: Echo
- **Database**: PostgreSQL — the source of truth. Time ranges are modelled as
  `tstzrange` with GiST indexes, so overlap detection is a database operation
  rather than application logic.
- **SQL**: `jet` (type-safe query builder), `sqlx`, `goose` for migrations
- **Cache / queues**: Valkey — idempotency keys, rate limiting
- **Background work**: a Postgres-backed task queue using
  `FOR UPDATE SKIP LOCKED`, so replicas never pick up the same task twice
- **Authorization**: Casbin
- **Observability**: Prometheus metrics, OpenTelemetry traces, structured logs

Design principles the code actually follows: clear separation between layers,
Postgres as the source of truth, time as a first-class domain concept, atomic
operations that behave correctly under concurrency, and as little magic as
possible.

## Getting started

Requirements: Go (version per `go.mod`), Docker with Compose, `make`.

```bash
make deps          # tool dependencies into ./bin
make bin-deps      # binary tools (goose, golangci-lint, mockgen, ...)
make secrets       # create deployment/maintmode/<env>/app.secrets.yaml from samples
make docker-up     # postgres, pg_doorman, valkey
make db-up         # apply migrations
make run           # start the service
```

`make air` runs the service with live reload and a Delve listener on `:2345`.

### Checks

```bash
make fmt           # gofmt + goimports
make lint          # golangci-lint
make tloc          # unit tests
make tloc-cov      # unit tests with coverage
make test-api      # API integration tests (brings up a compose stack)
```

## Configuration and secrets

Configuration and secrets are deliberately separate:

- `app.config.yaml` holds non-secret settings and *references* to secrets, written
  as `<secret:db/dsn>`.
- `app.secrets.yaml` is a flat key-value file, mounted at runtime and **never
  committed** — it is gitignored for every environment, including dev and local.
- Only `app.secrets.sample.yaml` files are tracked. They are templates for
  bootstrapping, not real values.

The service reads `app.config.yaml` and `app.secrets.yaml` from
`MAINTMODE_CONFIG_DIR`, and `model.conf` + `policy.csv` from `MAINTMODE_AUTHZ_DIR`.
File names can be overridden with `MAINTMODE_CONFIG_FILE` / `MAINTMODE_SECRETS_FILE`.
Unset variables default to the current directory.

```bash
MAINTMODE_CONFIG_DIR=deployment/maintmode/local
MAINTMODE_AUTHZ_DIR=deployment/maintmode/authz
```

In production, keep real values in a secret manager and mount a read-only
`app.secrets.yaml` into the container. The service does not call cloud-specific
secret APIs itself.

## First login

A fresh installation has no users. The first person to sign in through Google
becomes an administrator — that is how the initial account is created, and it
applies whenever the instance has no active administrators.

This is deliberately first-login-wins, with no locking. It assumes the operator
signs in before anyone else can reach the instance, so **sign in first, then
expose it**. After that, `allow_open_signup` stays `false` by default and further
users join by invitation.

## Licensing and telemetry

Two different things share the word "licence" here, so to be explicit:

**The code** is licensed under the GNU AGPL v3 — see [LICENSE](LICENSE).

**The commercial licence gate** in `internal/services/license` and
`internal/gateways/license` applies to the hosted SaaS offering, which charges by
seat. It is inert in self-hosted installations:

- The gate activates only when *both* `license.url` and `license.instance_token`
  are configured. A half-configured block stays off. Leave them unset — as the
  sample configs do — and there is no heartbeat, no seat limit, and no outbound
  request.
- With the gate off, nothing in this repository contacts an external service
  about your instance. There is no usage telemetry, no analytics, no phone-home.

The code stays in this repository rather than being stripped out because the same
binary serves both cases. You can read exactly what it does.

## Deployment

`compose.yaml` in this repository is for local development and CI: Postgres,
pg_doorman, Valkey, the app, Caddy, and the monitoring stack.

For running an instance:

- **Self-hosting** — see
  [maintmode-selfhost](https://github.com/maintmode-dev/maintmode-selfhost).
- **This repo's CI** publishes images to GHCR on every release.

### Scaling

The service is stateless and runs as N replicas:

```bash
make app-up MAINTMODE_REPLICAS=3
# or:
docker compose --profile storages --profile app up -d --scale maintmode=3
```

- Docker gives each replica a unique name; service-name DNS resolves to all of
  them, and Caddy load-balances with passive health checks and retries.
- The task queue uses `FOR UPDATE SKIP LOCKED`, so a task is never processed twice.
- Login rate limiting is shared through Valkey, with a per-replica in-memory
  fallback if Valkey is unreachable (alert: `RateLimiterValkeyFallback`).
- Prometheus discovers replicas through Docker service discovery; Promtail labels
  logs by container name.

Deploys are rolling: the app drains on `SIGTERM` (readiness flips to 503) while
Caddy's active `/readiness` check keeps the pool above its healthy count.

Postgres, Valkey, and Caddy remain single-instance — real HA needs infrastructure
beyond a single VM. After changing the replica count, restart Caddy so it
re-resolves DNS.

## Monitoring

The compose stack includes a full observability setup, all optional for
development:

| Service | Port | Purpose |
|---------|------|---------|
| Grafana | 8003 | Dashboards (default login `admin`/`admin`) |
| VictoriaMetrics | 8428 | Metrics, 30-day retention |
| Loki | 3100 | Log aggregation, 30-day retention |
| Tempo | — | Traces |
| Pyroscope | 4040 | Continuous profiling (internal only) |

Metrics come from the app (`echo_http_requests_total`,
`echo_http_request_duration_seconds`, Go runtime metrics) plus exporters for the
host, containers, Postgres, and Valkey. Profiling is pull-based: Grafana Alloy
scrapes the standard Go `pprof` endpoints and forwards them to Pyroscope — the
application pushes nothing and links no profiling SDK.

Configuration lives in `monitoring/config/`.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Security issues:
[SECURITY.md](SECURITY.md) — please do not open a public issue for those.

You may still see `RUK-123` identifiers in commit messages and in older
migration files. They reference the project's own issue tracker, which is not
public — nothing you need to read the code depends on them.

## Licence

[GNU Affero General Public License v3.0](LICENSE).

In short: you can run, modify, and distribute this, including commercially. If
you offer a modified version to others over a network, you have to make your
modified source available to them.

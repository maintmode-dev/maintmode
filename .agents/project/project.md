# Project Overview

## Product

MaintMode is a B2B maintenance calendar for engineering teams. It helps plan
and execute technical work without resource conflicts or unexpected production
incidents.

The core product idea:

> Time + shared resources = risk.

If two maintenance windows overlap and touch the same service, database, or
cluster, the system should make that risk explicit before work starts.

## Users

- Tech leads
- SRE and DevOps engineers
- Platform and backend teams
- Engineering organizations with shared infrastructure

## Core Capabilities

- Maintenance calendar for planned work
- Planned and actual execution windows
- Shared resource tracking
- Conflict detection by time and resources
- Risk visibility for reviewers and operators
- Notifications and workflow actions

## Architecture Principles

MaintMode is a **modular monolith**: one binary (`cmd/maintmode`), one
deployable process. Modules (core, auth, notificator, audit, integration,
license) are logical boundaries enforced by `depguard` in `.golangci.yaml`, not
separate services. See [workflow.md](./workflow.md) for how CI builds and
enforces this.

- Keep domain logic in services and entities.
- Keep HTTP binding and response shaping in API packages.
- Keep SQL in storage packages.
- Use PostgreSQL as the source of truth.
- Treat time ranges as domain data, not presentation details.
- Use transactions for multi-step state changes.
- Prefer predictable behavior over hidden magic.

## Stack

- Go 1.25+
- Echo for HTTP APIs
- PostgreSQL
- Jet and sqlx for typed SQL and DB access
- Goose for migrations
- Zap through `github.com/ruko1202/xlog`
- Redis outside the core domain for infrastructure concerns
- Docker-based local and deployment workflows

PostgreSQL-specific tools used by the project include `tstzrange` and GiST
indexes for time/resource conflict detection.

## Configuration And Secrets

Runtime config lives under `deployment/maintmode/<env>/`, where `<env>` is one
of `local`, `dev`, `test`, `prod` (`deployment/maintmode/authz` holds authz
policy). The sibling `deployment/` directories — `caddy`, `migrations`,
`infra` — are infrastructure, not application services.

- `app.config.yaml` contains non-secret configuration.
- `app.secrets.yaml` contains `<secret:key>` references.
- Runtime-mounted flat `secrets.yaml` files provide actual secret values.
- Real secret files must stay out of git; only sample templates are tracked.

## Philosophy

- Simplicity over generality.
- Predictability over magic.
- Prevention over reaction.
- Engineering honesty over feature count.

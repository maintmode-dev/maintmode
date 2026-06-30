# ADR-0003: Modular monolith over a microservice split

**Status:** Accepted
**Date:** 2026-06-18
**Deciders:** Ruslan (owner)

## Context

RUK-187 was framed as "split MaintMode into core, auth, and (maybe) notificator."
The codebase already runs two binaries (`cmd/maintmode`, `cmd/auth`) off one Go
module and one shared Postgres DB; core already calls auth over an S2S gateway
(`internal/gateways/auth/`). So a runtime boundary already exists — the question
RUK-187 really poses is **how far to push that boundary**, not whether to start
one.

Interrogating the existing split surfaced the real issue — and it is narrow:

- **The architecture itself is good.** Layers are clean, modules are separated,
  cross-module contracts are narrow consumer-side interfaces (see
  `architecture/architecture-as-is.md`). There is essentially nothing to fix in
  the structure.
- **Exactly one decision is excessive: drawing the core↔auth boundary over the
  network** (two processes + S2S via `internal/gateways/auth/`). This was a
  premature optimization — done on a "I thought I'd split it out" instinct, not a
  concrete need.
- The network boundary delivered **nothing** the monolith wouldn't have on a
  single VM (no independent scale, no deploy benefit, no second team) while
  **costing** real complexity: an S2S gateway to write and test, a split
  bootstrap, user resolution over the network instead of a call, and — when we
  designed the deeper split — the cross-DB audit atomicity problem.
- **No split trigger is on the horizon** (second team, independent scaling, data
  isolation/compliance, leaving the single VM). For notificator the owner's read
  is "maybe someday, but it lives here fine for now."

So this is not "the architecture is bad" — the as-is and target diagrams are
near-identical. It is "one excessive layer (the network boundary) should come
out for this stage."

Deployment is a single VM (Docker Compose + Caddy + Ansible, `ops/`), one owner,
internal MVP. Grafana — a far larger, busier system — is a **monolith**: auth,
RBAC, dashboards, datasources are modules in one process over one DB, talking via
in-memory interfaces, not a network. That is the relevant precedent.

The asymmetry that decides it: **monolith → services later** is cheap if module
boundaries are kept (it's a mechanical cut along a ready seam); **services →
monolith** is expensive (unwinding S2S, audit, data backfill — we felt this while
designing it). Under uncertainty about whether the split is ever needed, the
rational position is the cheap-to-reverse one.

## Decision

**Adopt a modular monolith as the target architecture. Collapse the prematurely
extracted auth binary back into a single process. Keep core / auth / notificator
/ audit as modules with strict package boundaries, communicating via in-memory
Go interfaces over one Postgres DB.** The full service split is preserved as a
documented future goal, triggered only by a concrete need.

- **One process, one binary.** auth folds back in (like `auth`/`accesscontrol`
  in Grafana). The S2S call site (`Introspector: gateways.Auth`) becomes a direct
  call to the local `user`/`token` services that already implement it; RBAC and
  token verification in core are already local (`services.RBAC`,
  `services.JWTVerifier`).
- **One database.** FKs and JOINs within the DB are fine where natural
  (notify→channels, invitations→users). The cross-DB couplings the service split
  had to untangle simply don't arise.
- **Audit stays simple.** Single `audit_log`, `audit.write` outbox in the same DB,
  one drainer. No audit plane, no S2S-ingest, no relay-outbox — the cross-DB
  atomicity problem disappears with the boundary.
- **Boundaries are machine-enforced, not willpower.** A CI import check (e.g.
  `depguard`/architecture test) forbids a module from importing another module's
  internals (`storages`/`entity`); only the module's public interface is
  importable. This is the safeguard that keeps a future split cheap.
- **The service split ([architecture/service-split.md](../../architecture/service-split.md))
  is parked, not discarded** — revisited per the triggers in
  [architecture.md](../../architecture/architecture.md) §7.

RUK-187 changes **no code** — it is design. Collapsing the auth binary and adding
the import-boundary enforcement land as follow-up issues.

## Options Considered

### Option A: Three services, separate DBs, S2S-only (the original RUK-187 plan)
Detailed in [service-split.md](../../architecture/service-split.md). **Pros:**
independent deploy/scale, hard data isolation, blast-radius isolation. **Cons:**
on a single VM with one owner none of those benefits are realized; pays
S2S-latency, eventual consistency, N databases, data backfill, and an audit plane
with S2S-ingest — complexity with no payoff at this stage.

### Option B: Modular monolith, auth folded back in (chosen)
Detailed in [architecture.md](../../architecture/architecture.md).
| Dimension | Assessment |
|-----------|------------|
| Processes | one |
| Module boundary | packages + DI, CI-enforced imports |
| DB | one |
| Cross-domain coupling | ordinary FK/JOIN; no untangling needed |
| Audit | single log, local outbox; no atomicity problem |
| Operational cost | low |
| Independent deploy/scale | no (added later per trigger) |
| Reversibility | split later along ready seams — cheap |

### Option C: Hybrid — auth stays a separate binary, core+notificator monolith
**Pros:** keeps auth's separate security perimeter. **Cons:** keeps the exact S2S
complexity we found unjustified, for a perimeter benefit not currently required.
Rejected because the auth perimeter is not a live requirement — if it becomes
one, that is itself a trigger (Option A territory).

## Trade-off Analysis

The decision is driven by stage, not by a belief that monoliths are universally
better. At one VM / one owner / internal MVP, the service split's benefits sit
idle while its costs are paid in full. The modular monolith delivers the *actual*
goal of RUK-187 — clean boundaries between core/auth/notificator — at the package
level, which is where the value was, while in-memory calls and one DB remove the
distributed-systems tax. The price is a shared failure radius (a panic anywhere
kills the process) and reliance on CI to keep boundaries honest; both are
acceptable on a single VM and the second is mitigated by the import check. The
original instinct ("draw boundaries") was right; only the *mechanism* (network +
separate DBs) was premature.

## Consequences

**Easier:** one process to deploy/operate; ordinary FK/JOIN; transactional
consistency; no S2S, no audit plane, no data backfill; clean module boundaries
that keep a future split cheap.
**Harder:** shared failure radius (process-wide); boundaries depend on a CI
import check rather than a network barrier — must be in place or the monolith
rots into a big ball of mud; auth shares a process with the domain (no separate
perimeter — acceptable now, becomes a trigger if not).
**Revisit:** move toward [service-split.md](../../architecture/service-split.md)
when a concrete trigger fires — second team needing independent release cadence,
notification delivery needing separate scale, data-isolation/compliance
requirement, or leaving the single VM.

## Action Items

1. [ ] [architecture.md](../../architecture/architecture.md) is the canonical
       target design; [service-split.md](../../architecture/service-split.md) is
       the parked future-split plan. Link both from the docs README.
2. [ ] Follow-up: add CI import-boundary enforcement
       (`depguard`/architecture test) forbidding cross-module internal imports.
3. [ ] Follow-up: collapse the `cmd/auth` binary into `cmd/maintmode` — replace
       the `gateways.Auth` S2S call with a direct local `user`/`token` call;
       merge bootstrap; drop `deployment/auth`, `ExternalServices`/`S2SConfig`.
4. [ ] Keep author/actor UUIDs FK-less and domain events on the outbox, so a
       future split stays a mechanical cut.
5. [ ] No backend code changes under RUK-187 itself.

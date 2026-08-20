# Security Policy

## Reporting a vulnerability

**Please do not open a public issue for security problems.**

Report privately through GitHub's
[private vulnerability reporting](https://github.com/maintmode-dev/maintmode/security/advisories/new)
("Security" tab → "Report a vulnerability"). This keeps the report visible only
to the maintainers until a fix is available.

Please include:

- what the issue is and roughly how severe you think it is,
- the steps to reproduce it, or a proof of concept,
- the version or commit you tested against,
- whether the instance was self-hosted or the hosted service.

You should get a first response within about a week. This is a small project,
so please treat that as a good-faith target rather than a guarantee.

## Supported versions

Fixes land on `main` and go out in the next release. There is no long-term
support branch: if you self-host, staying close to the latest release is the
supported path.

## Scope

In scope: this repository — the MaintMode backend, its API surface,
authentication and authorization, and the deployment manifests kept here.

Out of scope: findings that require an attacker to already control the host or
the database; issues in third-party dependencies (report those upstream, though
telling us is welcome); and anything that depends on deliberately insecure
configuration — for example running with `environment: local`, or with
`allow_open_signup: true` on an instance exposed to the internet.

## A note on self-hosted instances

You run your own instance, so its security is yours to operate. Two things
matter more than the rest:

- **The first login on a fresh installation becomes an administrator.** This is
  how the initial account is created, and it is deliberately first-login-wins
  with no locking. Log in yourself before the instance is reachable by anyone
  else.
- **Keep `allow_open_signup: false`** (the default) unless you specifically want
  anyone who can reach the instance to be able to register.

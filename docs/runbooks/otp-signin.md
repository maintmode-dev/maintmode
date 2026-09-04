# Runbook: email one-time-code sign-in

Covers `POST /api/v1/login/otp/request` and `POST /api/v1/login/otp/verify`.

Both endpoints answer every failure identically on purpose, so a user's report
("it says the code is wrong") rarely narrows anything down. The audit trail and
the logs are where the distinctions live.

## Limiter degradation

Three tiers guard these routes, and they degrade differently when Valkey is
unreachable:

| Tier | Key | On a Valkey outage |
| --- | --- | --- |
| Per IP | client address | per-replica bucket, so the effective cap is `N × limit` across `N` replicas |
| Per address | the email in the body | same, per replica |
| Instance-wide | a constant | **stops applying entirely** |

The instance-wide tier is the deliberate exception. Its key is a constant, so a
per-replica bucket would put every caller into one in-memory bucket and let a
single attacker hold the whole sign-in surface at 429. Losing the anti-sweep
control during an outage beats converting the outage into a total sign-in
outage; the per-address tier, which protects an individual account, keeps
working throughout.

The practical consequence: **during a Valkey outage this deployment is more
exposed to a distributed sweep than usual.** If an outage coincides with a spike
in failed verifications, treat the two as related.

## Reading the fallback signal

The alert is `RateLimiterValkeyFallback`, on `ratelimit_valkey_failures_total`.

That counter carries **no tier label**, so it cannot tell you which limiter fell
back. Since one Valkey outage is the common cause for all three, read a firing
alert as *all three degraded at once* — per-IP and per-address to `N × limit`,
instance-wide to nothing.

A sustained non-zero rate means Valkey, not an attack. Check Valkey's own health
first; the limiters recover on their own once it answers.

`otp_attempt_claim_errors_total` is separate and more serious. It counts
failures to claim a guess against a code, and the claim *is* the per-code
attempt ceiling. A sustained non-zero rate means the brute-force ceiling is not
being enforced — verification fails closed while this happens, so users see
errors rather than a silent hole, but the cause is a database problem and
nothing else will say so.

## Clearing the burnt-code barrier

Spending every guess on a code does **not** free the active-code slot: no new
code is issued until the burnt one expires (at most `auth.otp_ttl`, 5 minutes by
default). Without that rule, "five attempts" would mean "five attempts per code,
unlimited codes".

The refusal is deliberately invisible — the same `202` and the same response
shape as a successful request — and deliberately unaudited, since no secret was
presented and auditing it would let anyone write unbounded rows by replaying the
endpoint. So a barred user has no self-service diagnosis, and this is the only
way to confirm it.

Find a barred address:

```sql
SELECT c.id, u.email, c.attempts, c.expires_at
FROM auth_credentials c
JOIN users u ON u.id = c.user_id
WHERE c.kind = 'otp'
  AND c.consumed_at IS NULL
  AND c.expires_at > now()
  AND c.attempts >= 5          -- auth.otp_max_attempts
  AND u.email = 'user@example.com';
```

Clear it, **keyed on the id from that query**:

```sql
UPDATE auth_credentials SET consumed_at = now() WHERE id = '<id>';
```

The predicate is not optional. Running the `SET` without a `WHERE` retires every
live code on the instance: nobody is signed in by it, and everyone waiting on a
code has to request a new one.

Usually the right answer is to wait. The barrier is at most one `otp_ttl`, and
clearing it by hand for a user who is being targeted removes the control while
the attempt is in progress.

## Reading a 429 storm

A burst of 429s on `/login/otp` no longer means "one caller is hammering us".
With an instance-wide tier, one attacker exhausting the global budget returns
429 to **every** caller, including legitimate ones. Nothing in the metrics
distinguishes that from a local burst or from a Valkey fallback.

To tell them apart:

- Are the 429s spread across many client addresses? That points at the global
  tier, so a sweep is in progress and everyone is being refused.
- Concentrated on one address? Per-IP or per-address, and the ordinary cause.
- Is `RateLimiterValkeyFallback` firing? Then caps are per-replica and the
  instance-wide tier is off — the 429s are not coming from it.

Any alert added on this route later must exclude 429. The per-IP limiter carries
no route component, so `login/oauth`, `login/password` and `users/invitations`
share one budget with these routes and produce 429s here for traffic that never
touched them.

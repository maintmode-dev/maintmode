#!/usr/bin/env bash
#
# Verify that `make prod-deploy` is genuinely zero-downtime (RUK-147).
#
# It drives steady traffic with `hey` at the PUBLIC endpoint while a rolling
# deploy runs and asserts ZERO 5xx responses. It probes a path that reaches
# the app through Caddy but needs no auth token: an unauthenticated request
# returns 401 (a 4xx — the app answered), whereas a missing healthy upstream
# surfaces as 502/503 (a 5xx). So counting only 5xx isolates downtime from
# ordinary auth rejections.
#
# Two checks:
#   1. happy path  — run a real rolling deploy under load, expect 0 5xx.
#   2. failure path — start a replica on a broken image; the deploy must
#      abort and the OLD replicas must keep serving (still 0 5xx).
#
# Usage:
#   DOMAIN=maintmode.example.com ./scripts/verify-zero-downtime.sh
#   # against a local prod-like stack you can pass BASE_URL=http://localhost
#
# Env:
#   BASE_URL    full base URL to hit (default: https://$DOMAIN)
#   DOMAIN      public hostname (used when BASE_URL is unset)
#   DURATION    load duration, seconds (default 60)
#   QPS         requests per second (default 50)
#   COMPOSE / MAINTMODE_REPLICAS / AUTH_REPLICAS  passed to rolling-deploy.sh
#   BROKEN_IMAGE  image used for the failure-path test
#                 (default: alpine:latest — exits immediately, never ready)

set -euo pipefail

BASE_URL="${BASE_URL:-https://${DOMAIN:?set DOMAIN or BASE_URL}}"
DURATION="${DURATION:-60}"
QPS="${QPS:-50}"
PROBE_PATH="${PROBE_PATH:-/maintmode/api/v1/resources}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if ! command -v hey >/dev/null 2>&1; then
	echo "ERROR: 'hey' is not installed. Install it: go install github.com/rakyll/hey@latest" >&2
	exit 127
fi

log() { printf '\033[36m[verify]\033[0m %s\n' "$*"; }
pass() { printf '\033[32m[verify] PASS:\033[0m %s\n' "$*"; }
fail() {
	printf '\033[31m[verify] FAIL:\033[0m %s\n' "$*" >&2
	exit 1
}

# run_load runs hey for DURATION seconds, writing the full report to $1.
run_load() {
	local out="$1"
	hey -z "${DURATION}s" -q "$QPS" -c "$QPS" -disable-keepalive \
		"${BASE_URL}${PROBE_PATH}" >"$out" 2>&1 || true
}

# count_5xx sums every 5xx bucket in a hey report (0 if the section is absent).
# hey groups responses under a "Status code distribution" block.
count_5xx() {
	awk '
		/Status code distribution/ { b=1; next }
		b && /^[[:space:]]*\[5[0-9][0-9]\]/ { gsub(/[^0-9 ]/,""); s+=$1 }
		b && NF==0 { b=0 }
		END { print s+0 }
	' "$1"
}

# ---- Check 1: happy path -----------------------------------------------
log "Check 1/2 — steady traffic (${QPS} q/s for ${DURATION}s) during a real rolling deploy"
report1="$(mktemp)"
run_load "$report1" &
load_pid=$!

sleep 3 # let traffic ramp before we start moving replicas
log "  triggering rolling deploy..."
COMPOSE="${COMPOSE:-docker compose}" \
	MAINTMODE_REPLICAS="${MAINTMODE_REPLICAS:-2}" \
	AUTH_REPLICAS="${AUTH_REPLICAS:-2}" \
	"${SCRIPT_DIR}/rolling-deploy.sh" maintmode auth

wait "$load_pid"
five_xx="$(count_5xx "$report1")"
log "  full hey report:"
sed 's/^/    /' "$report1"
if [[ "$five_xx" -ne 0 ]]; then
	fail "happy path saw ${five_xx} 5xx responses during deploy (expected 0)"
fi
pass "happy path: 0 5xx during rolling deploy"

# ---- Check 2: failure path ---------------------------------------------
# A new replica that never becomes healthy must NOT take down the pool: the
# deploy aborts and the old replicas keep serving. FORCE_READINESS_FAIL=1
# makes rolling-deploy treat the freshly started replica as never-ready, so
# wait_ready times out and the abort/rollback branch runs for real — that is
# exactly the path a genuinely broken image would take.
log "Check 2/2 — failed new replica must not break the pool (old version keeps serving)"
report2="$(mktemp)"
run_load "$report2" &
load_pid=$!
sleep 3

log "  attempting a deploy whose new replica never becomes ready..."
set +e
COMPOSE="${COMPOSE:-docker compose}" \
	MAINTMODE_REPLICAS="${MAINTMODE_REPLICAS:-2}" \
	AUTH_REPLICAS="${AUTH_REPLICAS:-2}" \
	READINESS_TIMEOUT=15 \
	FORCE_READINESS_FAIL=1 \
	"${SCRIPT_DIR}/rolling-deploy.sh" maintmode
deploy_rc=$?
set -e
wait "$load_pid"

five_xx2="$(count_5xx "$report2")"
log "  full hey report:"
sed 's/^/    /' "$report2"

if [[ "$deploy_rc" -eq 0 ]]; then
	fail "failure-path deploy unexpectedly succeeded (broken replica should have aborted it)"
fi
if [[ "$five_xx2" -ne 0 ]]; then
	fail "failure path saw ${five_xx2} 5xx — a bad new replica took down the pool"
fi
pass "failure path: deploy aborted (rc=${deploy_rc}), 0 5xx, old version kept serving"

log "ALL CHECKS PASSED — rolling deploy is zero-downtime for maintmode + auth"

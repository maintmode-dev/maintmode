#!/usr/bin/env bash
#
# Zero-downtime rolling deploy for the maintmode / auth app services on
# single-VM Docker Compose (RUK-147).
#
# `docker compose up --scale` recreates every replica of a service at once,
# which leaves Caddy with no healthy upstream for a moment → 502s. This
# script instead rolls replicas ONE AT A TIME:
#
#   for each old replica:
#     1. scale the service up by one  → a new-image replica starts
#        (--no-recreate keeps the existing replicas untouched)
#     2. wait until the new replica's /readiness is 200
#     3. stop one OLD-image replica (SIGTERM) → it flips /readiness to 503,
#        Caddy's active health check ejects it within ~health_interval, then
#        it drains in-flight requests for shutdown.drain_timeout and exits
#     4. remove the stopped container so the replica count is back to N
#
# At every step at least N healthy replicas serve traffic, so steady load
# sees zero 5xx. If a new replica never becomes healthy the script aborts
# WITHOUT touching the old replicas — the running version keeps serving.
#
# It deliberately does NOT build a general orchestrator: it is a small,
# explicit loop over the two known app services. Caddy and storages are
# never touched (see Makefile prod-deploy / prod-migrate).
#
# Usage:
#   scripts/rolling-deploy.sh [service ...]      # default: maintmode auth
#
# Required env (normally exported by the Makefile prod-deploy target):
#   COMPOSE              full "docker compose --env-file ... -f ..." prefix
#   MAINTMODE_REPLICAS   target replica count for maintmode (default 2)
#   AUTH_REPLICAS        target replica count for auth (default 2)
# Optional:
#   READINESS_TIMEOUT    seconds to wait for a new replica (default 90)
#   DRAIN_WAIT           seconds to wait after stopping an old replica so
#                        Caddy fully ejects it (default 8; should exceed the
#                        app's shutdown.drain_timeout)
#   STOP_TIMEOUT         docker stop -t grace given to a retiring replica;
#                        must exceed shutdown.drain_timeout + HTTP graceful
#                        window (default 20)
#   INFRA_PORT           in-container infra/readiness port (default 8001)

set -euo pipefail

COMPOSE="${COMPOSE:-docker compose}"
MAINTMODE_REPLICAS="${MAINTMODE_REPLICAS:-2}"
AUTH_REPLICAS="${AUTH_REPLICAS:-2}"
READINESS_TIMEOUT="${READINESS_TIMEOUT:-90}"
DRAIN_WAIT="${DRAIN_WAIT:-8}"
STOP_TIMEOUT="${STOP_TIMEOUT:-20}"
INFRA_PORT="${INFRA_PORT:-8001}"

SERVICES=("$@")
if [[ ${#SERVICES[@]} -eq 0 ]]; then
	SERVICES=(maintmode auth)
fi

log() { printf '\033[36m[rolling-deploy]\033[0m %s\n' "$*"; }
err() { printf '\033[31m[rolling-deploy] ERROR:\033[0m %s\n' "$*" >&2; }

# replica_count returns the configured target replica count for a service.
replica_count() {
	case "$1" in
	maintmode) echo "$MAINTMODE_REPLICAS" ;;
	auth) echo "$AUTH_REPLICAS" ;;
	*)
		err "unknown service '$1' (no replica count configured)"
		exit 2
		;;
	esac
}

# container_ids lists running container IDs for a compose service, oldest first.
container_ids() {
	$COMPOSE ps -q "$1"
}

# image_of prints the image ID a container was created from.
image_of() {
	docker inspect --format '{{.Image}}' "$1"
}

# wait_ready blocks until the container's /readiness returns 200, or fails.
wait_ready() {
	local cid="$1" deadline=$((SECONDS + READINESS_TIMEOUT))
	# Test hook: pretend the new replica never comes up so the abort/rollback
	# path can be exercised (see scripts/verify-zero-downtime.sh check 2).
	if [[ "${FORCE_READINESS_FAIL:-0}" == "1" ]]; then
		log "  FORCE_READINESS_FAIL set — treating ${cid:0:12} as never-ready"
		sleep "$READINESS_TIMEOUT"
		return 1
	fi
	log "  waiting for new replica ${cid:0:12} to become ready (timeout ${READINESS_TIMEOUT}s)..."
	while ((SECONDS < deadline)); do
		# Probe from inside the container so we do not depend on host port
		# mappings (replicas have none) — wget ships in the alpine image.
		if docker exec "$cid" wget --no-verbose --tries=1 --spider \
			"http://localhost:${INFRA_PORT}/readiness" >/dev/null 2>&1; then
			log "  new replica ${cid:0:12} is ready"
			return 0
		fi
		sleep 2
	done
	return 1
}

# roll_service replaces every old-image replica of one service, one at a time.
roll_service() {
	local svc="$1"
	local target
	target="$(replica_count "$svc")"

	log "rolling '$svc' (target ${target} replicas)..."

	# Snapshot the image the new replicas will use (compose already pulled it)
	# and the set of currently-running OLD replicas to retire.
	local new_image
	new_image="$($COMPOSE config --images "$svc" 2>/dev/null | head -n1)"
	local old_ids=()
	local id
	while IFS= read -r id; do
		[[ -n "$id" ]] && old_ids+=("$id")
	done < <(container_ids "$svc")

	if [[ ${#old_ids[@]} -eq 0 ]]; then
		log "  no running replicas — starting $target fresh"
		$COMPOSE up -d --no-deps --no-build --scale "$svc=$target" "$svc"
		return 0
	fi

	# `known` tracks every container ID we've already seen for this service, so
	# after each scale-up the single ID that is NOT in `known` is the replica
	# compose just added — robust across multiple iterations (the new replica
	# from a previous step is already in `known`, so it is never re-picked).
	local known=("${old_ids[@]}")

	local rolled=0
	for old in "${old_ids[@]}"; do
		# Scale up by one: compose adds a single new-image replica and, with
		# --no-recreate, leaves every existing replica running.
		local scale_to=$((target + 1))
		log "  [$((rolled + 1))/${#old_ids[@]}] scaling '$svc' up to ${scale_to} (adding a new-version replica)"
		$COMPOSE up -d --no-deps --no-build --no-recreate --scale "$svc=$scale_to" "$svc"

		# The freshly added container is the one running ID not yet in `known`.
		local new_cid=""
		local cid
		while IFS= read -r cid; do
			[[ -z "$cid" ]] && continue
			local seen=0
			for k in "${known[@]}"; do [[ "$cid" == "$k" ]] && seen=1 && break; done
			if [[ $seen -eq 0 ]]; then new_cid="$cid"; fi
		done < <(container_ids "$svc")

		if [[ -z "$new_cid" ]]; then
			err "could not find the newly started replica for '$svc'; aborting (old replicas untouched)"
			exit 1
		fi
		known+=("$new_cid")

		if ! wait_ready "$new_cid"; then
			err "new replica ${new_cid:0:12} for '$svc' never became ready"
			err "aborting deploy — removing the failed replica, OLD version keeps serving"
			docker rm -f "$new_cid" >/dev/null 2>&1 || true
			$COMPOSE up -d --no-deps --no-build --no-recreate --scale "$svc=$target" "$svc" || true
			exit 1
		fi

		# Retire one old replica from the ORIGINAL snapshot (never a new one),
		# addressed by its container ID so compose's index reuse can't make us
		# stop the wrong container. SIGTERM triggers the in-app drain (readiness
		# 503 → Caddy ejects → in-flight requests finish); -t must exceed the
		# app's drain_timeout + HTTP graceful window so the stop is clean.
		if ! docker inspect "$old" >/dev/null 2>&1; then
			err "old replica ${old:0:12} for '$svc' vanished before we could retire it; aborting"
			exit 1
		fi
		log "  retiring old replica ${old:0:12} (SIGTERM → drain → exit)"
		docker stop -t "$STOP_TIMEOUT" "$old" >/dev/null
		docker rm "$old" >/dev/null
		log "  waiting ${DRAIN_WAIT}s for Caddy to settle the pool..."
		sleep "$DRAIN_WAIT"

		rolled=$((rolled + 1))
	done

	# Normalise: ensure exactly `target` replicas remain (no-op if already so).
	$COMPOSE up -d --no-deps --no-build --no-recreate --scale "$svc=$target" "$svc"
	log "'$svc' rolled to the new version (${target} replicas), image ${new_image}"
}

for svc in "${SERVICES[@]}"; do
	roll_service "$svc"
done

log "rolling deploy complete for: ${SERVICES[*]}"

# -------------------------------------
# Production single-VM deployment
# -------------------------------------
# Targets for running MaintMode on a single cloud VM via Docker Compose.
# Layers compose.app.prod.yaml on top of compose.yaml to swap dev
# configs for prod, switch to Caddy with TLS, and pin published image
# tags from .env.prod.
#
# Included from the root Makefile. Depends on COMPOSE_PROFILES_FLAGS
# being defined in the parent.
#
# Prereqs:
#   - .env.prod created from .env.prod.sample
#   - deployment/{auth,maintmode}/prod/app.secrets.yaml populated
#   - DNS A record for $DOMAIN points to this VM
#
# Documentation: maintmode-docs/ops/vm-provisioning.md

PROD_COMPOSE_CONFIGS ?= -f compose.yaml -f compose.app.prod.yaml
PROD_ENV_FILE ?= .env.prod

# Replica counts for the two scalable app services. prod-deploy keeps these
# constant while rolling the new image version through one replica at a time.
PROD_MAINTMODE_REPLICAS ?= 2
PROD_AUTH_REPLICAS ?= 2

# Full compose invocation reused by the deploy targets and handed to the
# rolling-deploy script so it talks to the exact same project / overlays.
PROD_COMPOSE = docker-compose --env-file $(PROD_ENV_FILE) $(PROD_COMPOSE_CONFIGS)

.PHONY: prod-up
prod-up: ## Start the full production stack on this VM
	$(info $(M) starting production stack...)
	$(PROD_COMPOSE) ${COMPOSE_PROFILES_FLAGS} up -d \
		--scale maintmode=$(PROD_MAINTMODE_REPLICAS) \
		--scale auth=$(PROD_AUTH_REPLICAS)
	make prod-ps

# prod-migrate — apply goose migrations as a one-shot, independent of the
# code deploy. Run this BEFORE prod-deploy whenever a release adds
# migrations. Migrations must be expand-contract (additive first) so the
# still-running old code is not broken while the new code rolls out — see
# maintmode-docs/ops/rolling-deploy.md.
.PHONY: prod-migrate
prod-migrate: ## Apply DB migrations (one-shot goose, no code deploy)
	$(info $(M) applying migrations...)
	$(PROD_COMPOSE) --profile storages run --rm apply-migrations

# prod-deploy — ship a new image version of maintmode/auth with zero 502s.
# Pulls the tags pinned in .env.prod, then rolls each app service through
# one replica at a time (scripts/rolling-deploy.sh). Never touches storages
# or Caddy. If a new replica fails to become healthy the deploy aborts and
# the old version keeps serving. Run prod-migrate first if there are new
# migrations.
.PHONY: prod-deploy
prod-deploy: ## Rolling redeploy of app services (zero-downtime)
	$(info $(M) pulling new app images...)
	$(PROD_COMPOSE) --profile app pull maintmode auth
	$(info $(M) rolling deploy maintmode + auth...)
	COMPOSE="$(PROD_COMPOSE)" \
		MAINTMODE_REPLICAS=$(PROD_MAINTMODE_REPLICAS) \
		AUTH_REPLICAS=$(PROD_AUTH_REPLICAS) \
		./scripts/rolling-deploy.sh maintmode auth
	make prod-ps

# prod-verify — drive steady traffic with `hey` against the public endpoint
# while a deploy runs and assert zero 5xx. See scripts/verify-zero-downtime.sh.
.PHONY: prod-verify
prod-verify: ## Load-test zero-downtime deploy (hey, asserts 0 5xx)
	$(info $(M) verifying zero-downtime deploy...)
	DOMAIN=$$(grep -E '^DOMAIN=' $(PROD_ENV_FILE) | cut -d= -f2) \
		COMPOSE="$(PROD_COMPOSE)" \
		MAINTMODE_REPLICAS=$(PROD_MAINTMODE_REPLICAS) \
		AUTH_REPLICAS=$(PROD_AUTH_REPLICAS) \
		./scripts/verify-zero-downtime.sh

.PHONY: prod-down
prod-down: ## Stop the production stack (volumes preserved)
	$(info $(M) stopping production stack...)
	docker-compose --env-file $(PROD_ENV_FILE) ${PROD_COMPOSE_CONFIGS} ${COMPOSE_PROFILES_FLAGS} down --remove-orphans

.PHONY: prod-logs
prod-logs: service=
prod-logs: ## Follow logs from prod services (override via service=...)
	docker-compose --env-file $(PROD_ENV_FILE) ${PROD_COMPOSE_CONFIGS} ${COMPOSE_PROFILES_FLAGS} logs -f $(service)

.PHONY: prod-ps
prod-ps: ## Show status of prod containers
	docker-compose --env-file $(PROD_ENV_FILE) ${PROD_COMPOSE_CONFIGS} ${COMPOSE_PROFILES_FLAGS} ps -a

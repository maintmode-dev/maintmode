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

.PHONY: prod-up
prod-up: ## Start the full production stack on this VM
	$(info $(M) starting production stack...)
	docker-compose --env-file $(PROD_ENV_FILE) ${PROD_COMPOSE_CONFIGS} ${COMPOSE_PROFILES_FLAGS} up -d
	make prod-ps

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

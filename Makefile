# -------------------------------------
# Production single-VM deployment
# -------------------------------------
# prod-up / prod-down / prod-logs / prod-ps live in makefile.prod.mk
# so the root Makefile stays focused on dev workflows.
-include makefile.prod.mk

# -------------------------------------
# Shared Configuration
# -------------------------------------
# This file contains common configuration variables shared across all Makefiles.
# It is automatically included by Makefile and database.mk.


# GOBIN - Directory for installed Go binaries (goose, golangci-lint, mockgen, etc.)
# Default: ./bin in the project root
GOBIN			?= $(PWD)/bin

# SECRETS_FILE - Path to secret file
SECRETS_FILE ?= $(PWD)/deployment/maintmode/local/app.secrets.yaml

# -------------------------------------
# Database Configuration
# -------------------------------------
# Migration directories for each database type
# These paths are relative to ROOT_DIR (project root)
MIGRATIONS_DIR	?= migrations
DB_DRIVER		?= postgres
DB_DSN			?=

ifndef DB_DSN
DB_DSN=$(shell [ -f "$(SECRETS_FILE)" ] && awk '/^"?db\/dsn"?:/ {value=$$0; sub(/^[^:]+:[[:space:]]*/, "", value); gsub(/^"/, "", value); gsub(/"$$/, "", value); print value; exit}' "$(SECRETS_FILE)" || echo "")
endif

# Ensure GOBIN directory exists
$(shell mkdir -p $(GOBIN))
$(shell mkdir -p ./tmp)

DOCKER_COMPOSE_APP_CONFIGS ?= -f compose.yaml

# Compose profiles. After RUK-21, every service in compose.yaml declares a
# `profiles:` key, so any compose command that should start services must
# pass these flags explicitly. Override COMPOSE_PROFILES_FLAGS=... for
# selective runs (e.g. test-api drops monitoring).
COMPOSE_PROFILES_FLAGS ?= --profile storages --profile app --profile monitoring
COMPOSE_PROFILES_STORAGES ?= --profile storages

# -------------------------------------
# Default target
# -------------------------------------
# Runs the complete build pipeline: install dependencies, binary tools,
# run tests, format code, and run linter checks
.PHONY: all
all: deps bin-deps tloc fmt lint build

# -------------------------------------
# Install deps and tools
# -------------------------------------

# deps - Download and organize Go module dependencies
# Downloads all required Go modules and removes unused ones
.PHONY: deps
deps:
	go mod download
	go mod tidy

# bin-deps-build - Install build-related binary tools
.PHONY: bin-deps-build
bin-deps-build:
	GOBIN=$(GOBIN) go install github.com/goreleaser/goreleaser/v2@v2.13.1

# bin-deps - Install all required binary tools
# Installs:
#   - goose: database migration tool
#   - golangci-lint: comprehensive Go linter
#   - mockgen: mock code generator for testing
#   - swag (v2), oapi-codegen: OpenAPI spec + client generators
.PHONY: bin-deps
bin-deps: bin-deps-build
	GOBIN=$(GOBIN) go install github.com/pressly/goose/v3/cmd/goose@v3.26.0 && \
	GOBIN=$(GOBIN) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2 && \
	GOBIN=$(GOBIN) go install go.uber.org/mock/mockgen@v0.6.0 && \
	GOBIN=$(GOBIN) go install github.com/go-delve/delve/cmd/dlv@v1.26.0 && \
	GOBIN=$(GOBIN) go install github.com/air-verse/air@v1.64.3 && \
	GOBIN=$(GOBIN) go install github.com/swaggo/swag/v2/cmd/swag@v2.0.0-rc5 && \
	GOBIN=$(GOBIN) go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.7.0

# -------------------------------------
# Build binary or run app
# -------------------------------------
.PHONY: run
run: service=maintmode
run: config_dir=$(PWD)/deployment/${service}/local
run: authz_dir=$(PWD)/deployment/${service}/authz
run:
	$(shell echo ${service} | tr 'a-z' 'A-Z')_CONFIG_DIR=${config_dir} \
		$(shell echo ${service} | tr 'a-z' 'A-Z')_AUTHZ_DIR=${authz_dir} \
		go run ./cmd/${service}

.PHONY: air
air: service=maintmode
air: config_dir=$(PWD)/deployment/${service}/local
air: authz_dir=$(PWD)/deployment/${service}/authz
air:
	$(shell echo ${service} | tr 'a-z' 'A-Z')_CONFIG_DIR=${config_dir} \
		$(shell echo ${service} | tr 'a-z' 'A-Z')_AUTHZ_DIR=${authz_dir} \
		$(GOBIN)/air

.PHONY: build
build: service=maintmode
build: args=--id main --output=$(GOBIN)/${service}
build: config=$(PWD)/deployment/${service}/.build/.goreleaser.yaml
build:
	$(GOBIN)/goreleaser build -f ${config} --snapshot --single-target --clean ${args}

.PHONY: build-dev
build-dev: swag
build-dev: service=maintmode
build-dev:
	make build service=${service} args="--id dev --output=$(GOBIN)/dev-${service}"


# -------------------------------------
# Tests
# -------------------------------------

# tloc - Run all tests with race detection disabled
# Options:
#   -p 2: Run tests in parallel with max 2 processes
#   -count 2: Run each test 2 times to catch flaky tests
# Note: Package tests are run from project root
.PHONY: tloc
tloc:
	MAINTMODE_CONFIG_DIR=$(PWD)/deployment/maintmode/local \
	MAINTMODE_AUTHZ_DIR=$(PWD)/deployment/maintmode/authz \
	AUTH_CONFIG_DIR=$(PWD)/deployment/auth/local \
	AUTH_AUTHZ_DIR=$(PWD)/deployment/auth/authz \
		go test -p 2 -count 2 ./internal/...

# tloc-cov - Run tests with coverage analysis
# Generates coverage report excluding mock files
# Options:
#   -race: Enable data race detection
#   -p 2: Run tests in parallel with max 2 processes
#   -count 2: Run each test 2 times
#   -coverprofile: Output file for coverage data
#   -covermode atomic: Use atomic mode for race detector compatibility
#   --coverpkg: Measure coverage for internal/ packages only
# Output files:
#   coverage.tmp: Raw coverage data
#   coverage.out: Filtered coverage data (no mocks)
#   coverage.report: Human-readable coverage report
.PHONY: tloc-cov
tloc-cov:
	MAINTMODE_CONFIG_DIR=$(PWD)/deployment/maintmode/local \
	MAINTMODE_AUTHZ_DIR=$(PWD)/deployment/maintmode/authz \
	AUTH_CONFIG_DIR=$(PWD)/deployment/auth/local \
	AUTH_AUTHZ_DIR=$(PWD)/deployment/auth/authz \
		go test -race -p 2 -count 2 -coverprofile=coverage.tmp -covermode atomic --coverpkg=./internal/... ./internal/...
	@grep -vE "mock|internal/pkg/generated" coverage.tmp > coverage.out
	go tool cover -func=coverage.out | sed 's|github.com/ruko1202/goque||' | sed -E 's/\t+/\t/g' | tee coverage.report


.PHONY: tloc-api
tloc-api: ## Run API integration tests
	$(info $(M) running API integration tests...)
	go test -tags=api -v -p 2 -count=2 ./test/api/...


.PHONY: test-api
test-api: ## Run API integration tests like in CI
	$(info $(M) running API integration tests...)
	DOCKER_COMPOSE_APP_CONFIGS="-f compose.yaml -f compose.app.test.yaml" \
		COMPOSE_PROFILES_FLAGS="--profile storages --profile app" \
		make app-up
	go test -tags=api -v -p 2 -count=2 ./test/api/...

.PHONY: tloc-all
tloc-all:
	make tloc
	make tloc-api

# -------------------------------------
# Linter and formatter
# -------------------------------------

# lint - Run comprehensive linter checks
# Uses golangci-lint with configuration from .golangci.yml
# Checks for code quality, style, bugs, and best practices
# Includes files with 'api' build tag (test/api directory)
.PHONY: lint
lint:
	$(info $(M) running linter...)
	@$(GOBIN)/golangci-lint run --build-tags=api ./...

# fmt - Format code and organize imports
# Uses gofmt for code formatting and goimports for import organization
# Automatically fixes formatting issues
.PHONY: fmt
fmt:
	$(info $(M) fmt project...)
	@# Format code with gofmt and organize imports with goimports
	@$(GOBIN)/golangci-lint fmt -E gofmt -E goimports ./...

# -------------------------------------
# Code generation
# -------------------------------------

# mocks - Generate mock implementations for testing
# Regenerates all mock files for processor interfaces
# Mocks are used for unit testing with go.uber.org/mock
# Generated files:
#   - mock_processors/queueprocessor/processor.go
#   - mock_processors/queueprocessor/task_processor.go
.PHONY: mocks
mocks:
	rm -rf ./internal/pkg/generated/mocks
	$(GOBIN)/mockgen -typed -destination ./internal/pkg/generated/mocks/dbtx/dbtx.go -source ./internal/utils/dbtx/main_test.go
	$(GOBIN)/mockgen -typed -destination ./internal/pkg/generated/mocks/services/oauthprovider/provider.go -source ./internal/services/oauthprovider/provider.go
	$(GOBIN)/mockgen -typed -destination ./internal/pkg/generated/mocks/services/messagesender/service.go -source ./internal/services/messaging/sender/service.go
	$(GOBIN)/mockgen -typed -destination ./internal/pkg/generated/mocks/services/maintnotify/service.go -source ./internal/services/maintnotify/service.go
	$(GOBIN)/mockgen -typed -destination ./internal/pkg/generated/mocks/services/deferrednotifications/service.go -source ./internal/services/deferrednotifications/service.go
	$(GOBIN)/mockgen -typed -destination ./internal/pkg/generated/mocks/server/middlewares/auth.go -source ./internal/server/middlewares/auth.go
	$(GOBIN)/mockgen -typed -destination ./internal/pkg/generated/mocks/server/middlewares/active_token.go -source ./internal/server/middlewares/active_token.go
	$(GOBIN)/mockgen -typed -destination ./internal/pkg/generated/mocks/server/middlewares/rbac.go -source ./internal/server/middlewares/rbac.go

# swag - Generate OpenAPI 3.1 specs from annotations, split by @Tags into the
# maintmode and auth services (yaml/json only, no docs.go). inject_servers adds
# the `servers:` block: local API on :8000 (run on host), dev API on :9000
# (container behind Caddy; {domain} filled in Swagger UI).
.PHONY: swag
swag:
	$(info $(M) generating OpenAPI specs (swag/v2)...)
	$(GOBIN)/swag init \
		-g ./doc_maintmode.go --parseInternal --parseDependency \
		--tags Maintenances,Resources,Notifications,UI \
		--v3.1 --outputTypes yaml,json -o ./docs/maintmode
	$(GOBIN)/swag init \
		-g ./doc_auth.go --parseInternal --parseDependency \
		--tags Auth,Roles,Audit \
		--v3.1 --outputTypes yaml,json -o ./docs/auth
	go run ./scripts/swaggerspec/inject_servers.go --spec ./docs/maintmode/swagger.yaml \
		--domain-default 'localhost' \
		--server 'http://{domain}:9000/maintmode|Dev (service in container)' \
		--server 'http://localhost:8000|Local (service run on host)'
	go run ./scripts/swaggerspec/inject_servers.go --spec ./docs/auth/swagger.yaml \
		--domain-default 'localhost' \
		--server 'http://{domain}:9000/auth|Dev (service in container)' \
		--server 'http://localhost:8000|Local (service run on host)'
	@make test-client

# test-client - Generate typed API test clients from the OpenAPI specs.
# Uses oapi-codegen; one client package per service under test/api/client/.
# Run this after `make swag` (or just run `make swag`, which chains it).
.PHONY: test-client
test-client: ## Generate API test clients from OpenAPI specs
	$(info $(M) generating API test clients (oapi-codegen)...)
	$(GOBIN)/oapi-codegen \
		-config ./test/api/client/maintmode/oapi-codegen.yaml ./docs/maintmode/swagger.yaml
	$(GOBIN)/oapi-codegen \
		-config ./test/api/client/auth/oapi-codegen.yaml ./docs/auth/swagger.yaml
	@echo "API clients generated in test/api/client/{maintmode,auth}/"

# -------------------------------------
# Database - Universal Commands (use DB_DRIVER)
# -------------------------------------
# These commands automatically work with any configured database.
# The actual database used is determined by DB_DRIVER variable.

# db-migrate-create - Create a new migration file
# Usage: make db-migrate-create name="add_users_table"
# Creates timestamped migration files in the appropriate migrations/ directory
# Format: YYYYMMDDHHMMSS_<name>.sql
.PHONY: db-migrate-create
db-migrate-create: name=
db-migrate-create: ## Create new migration for current DB_DRIVER ($(DB_DRIVER))
	$(info $(M) creating $(DB_DRIVER) migration...)
	$(GOBIN)/goose -dir $(MIGRATIONS_DIR) $(DB_DRIVER) "$(DB_DSN)" create "${name}" sql

# db-status - Show migration status
# Displays which migrations have been applied and which are pending
# Shows version number and migration name for each migration
.PHONY: db-status
db-status: ## Check migrations status for current DB_DRIVER ($(DB_DRIVER))
	$(info $(M) check $(DB_DRIVER) migrations status...)
	$(GOBIN)/goose -dir $(MIGRATIONS_DIR) $(DB_DRIVER) "$(DB_DSN)" status

# db-up - Apply all pending migrations
# Runs all unapplied migrations in sequential order
# For PostgreSQL: also regenerates type-safe models after migrations
# Safe to run multiple times (idempotent)
.PHONY: db-up
db-up: ## Apply migrations for current DB_DRIVER ($(DB_DRIVER))
	$(info $(M) starting $(DB_DRIVER) migration up...)
	$(GOBIN)/goose -dir $(MIGRATIONS_DIR) $(DB_DRIVER) "$(DB_DSN)" up
	$(MAKE) db-status

# db-down - Rollback the last migration
# Reverts the most recently applied migration
# Useful for undoing mistakes or testing rollback logic
# WARNING: This modifies the database schema
.PHONY: db-down
db-down: ## Rollback migrations for current DB_DRIVER ($(DB_DRIVER))
	$(info $(M) starting $(DB_DRIVER) migration down...)
	$(GOBIN)/goose -dir $(MIGRATIONS_DIR) $(DB_DRIVER) "$(DB_DSN)" down
	$(MAKE) db-status

# db-models - Generate type-safe database models (PostgreSQL only)
# Uses go-jet to generate Go structs and query builders from database schema
# Only works with PostgreSQL
# Generated files are placed in internal/pkg/generated/
.PHONY: db-models
db-models: ## Generate models
	$(info $(M) generating $(DB_DRIVER) models...)
	@go run ./scripts/dbmodels/generate.go --driver=$(DB_DRIVER) --dsn=$(DB_DSN) --dest="internal/pkg/generated/"


# -------------------------------------
# Docker Compose Commands
# -------------------------------------
# Convenient commands for managing database containers via Docker Compose
# Uses docker-compose.yml in project root
#
# Services defined:
#   - postgres: PostgreSQL 17 on port 5432
#
# All services include:
#   - Health checks for readiness detection
#   - Persistent volumes for data storage
#   - Standard test credentials (see docker-compose.yml)

# docker-up - Start all database services
# Starts both PostgreSQL in background (detached mode)
# Creates containers, networks, and volumes if they don't exist
# Safe to run multiple times (idempotent)
.PHONY: docker-up
docker-up: ## Start all databases with Docker Compose
	$(info $(M) starting databases with Docker Compose...)
	docker-compose ${COMPOSE_PROFILES_STORAGES} up -d
	make docker-ps

# docker-down - Stop and remove all database containers
# Stops containers, removes them, and deletes associated volumes
# WARNING: This will delete all data stored in the databases
# Use this when you want a fresh start with empty databases
.PHONY: docker-down
docker-down: ## Stop and remove all database containers with volumes
	$(info $(M) stopping all database containers and removing volumes...)
	docker-compose ${COMPOSE_PROFILES_STORAGES} down -v --remove-orphans
	make docker-ps

.PHONY: docker-reup
docker-reup:
	make docker-down
	make docker-up

# docker-logs - Stream logs from all database containers
# Shows continuous log output from both PostgreSQL
# Press Ctrl+C to stop following logs
# Useful for debugging connection issues or monitoring queries
.PHONY: docker-logs
docker-logs: ## Show logs from all database containers
	docker-compose ${COMPOSE_PROFILES_STORAGES} logs -f

# docker-ps - Show status of database containers
# Displays:
#   - Container name and status (running/stopped)
#   - Ports mapping
#   - Health check status
# Useful for verifying that databases are ready to accept connections
.PHONY: docker-ps
docker-ps: ## Show status of database containers
	docker-compose ${COMPOSE_PROFILES_STORAGES} ps -a

# -------------------------------------
# Docker Compose Commands with App
# -------------------------------------
# Commands for managing the full application stack via Docker Compose.
# Uses compose.yaml in project root with profiles (storages/app/monitoring).
#
# Services activated by COMPOSE_PROFILES_FLAGS (default = all three):
#   storages    postgres, pg_doorman, redis, apply-migrations
#   app         maintmode, auth, caddy
#   monitoring  VictoriaMetrics, Grafana, Loki, Promtail, exporters,
#               Jaeger, OTEL, Pyroscope, Alloy, vmalert, alertmanager

# app-up - Start all services including maintmode application
# Starts PostgreSQL, Redis, pg_doorman, apply-migrations, and maintmode
# Creates containers, networks, and volumes if they don't exist
# Safe to run multiple times (idempotent)
#
# Replica count for the two scalable services defaults to 1 (current
# behaviour). Override for multi-pod runs:
#   make app-up MAINTMODE_REPLICAS=3 AUTH_REPLICAS=3
# Restart caddy after changing the count so it re-resolves DNS.
MAINTMODE_REPLICAS ?= 1
AUTH_REPLICAS ?= 1
.PHONY: app-up
app-up: app-down
app-up: args=
app-up: ## Start all services with maintmode using Docker Compose
	$(info $(M) starting stack with maintmode=$(MAINTMODE_REPLICAS) auth=$(AUTH_REPLICAS)...)
	docker-compose ${DOCKER_COMPOSE_APP_CONFIGS} ${COMPOSE_PROFILES_FLAGS} up -d \
		--scale maintmode=$(MAINTMODE_REPLICAS) \
		--scale auth=$(AUTH_REPLICAS) \
		${args}
	make app-ps

# app-down - Stop and remove all containers
# Stops all containers and removes them
# WARNING: This will remove all containers but keeps volumes
# Use this when you want to stop services but preserve data
.PHONY: app-down
app-down: ## Stop and remove all containers
	$(info $(M) stopping all containers...)
	docker-compose ${DOCKER_COMPOSE_APP_CONFIGS} ${COMPOSE_PROFILES_FLAGS} down -v --remove-orphans
	make app-ps

.PHONY: app-reup
app-reup: ## Stop and start all containers
	make app-down
	make app-up args="--build"

# app-logs - Stream logs from maintmode container
# Shows continuous log output from maintmode service
# Press Ctrl+C to stop following logs
# Useful for debugging application issues
.PHONY: app-logs
app-logs: ## Show logs from maintmode container
	docker-compose ${DOCKER_COMPOSE_APP_CONFIGS} ${COMPOSE_PROFILES_FLAGS} logs -f maintmode

# app-ps - Show status of all containers
# Displays:
#   - Container name and status (running/stopped)
#   - Ports mapping
#   - Health check status
# Useful for verifying that all services are running
.PHONY: app-ps
app-ps: ## Show status of all containers
	docker-compose ${DOCKER_COMPOSE_APP_CONFIGS} ${COMPOSE_PROFILES_FLAGS} ps -a

# -------------------------------------
# K6 Load Testing
# -------------------------------------
# Commands for running K6 load tests against the API
# Tests are located in test/k6/scenarios/
#
# Environment variables:
#   BASE_URL: API base URL (default: http://maintmode:8000)
#   TEST: Specific test file to run (default: full scenario)
#
# Prerequisites:
#   - API server must be running (make app-up)
#   - Docker Compose must be available

# K6 Docker Compose configuration
K6_COMPOSE_FILE = compose.k6.yaml
K6_TEST_DIR = test/k6
K6_SCENARIOS_DIR = $(K6_TEST_DIR)/scenarios

# k6 - Run K6 tests using Docker Compose
# Usage:
#   make k6                                    		# Run full scenario test
#   make k6 TEST=scenarios/01-resources-test.js  	# Run specific test
.PHONY: k6
k6: TEST?=scenarios/00-full-scenario-test.js
k6: ## Run K6 load tests using Docker Compose
	$(info $(M) running K6 load test: $(TEST)...)
	@docker-compose -f $(K6_COMPOSE_FILE) run --rm \
		-e BASE_URL=http://maintmode:8000 \
		k6 run /k6/$(TEST) \
		--out experimental-prometheus-rw \
		--tag testid=k6-load-test

# k6-all - Run all K6 tests sequentially using Docker Compose
.PHONY: k6-all
k6-all: ## Run all K6 tests using Docker Compose
	$(info $(M) running all K6 tests...)
	make k6 TEST=scenarios/00-full-scenario-test.js  # Run full scenario test
	make k6 TEST=scenarios/01-resources-test.js # Run resources API test
	make k6 TEST=scenarios/02-maintenances-crud-test.js # Run maintenances CRUD test
	make k6 TEST=scenarios/03-maintenances-lifecycle-test.js # Run maintenances lifecycle test
	make k6 TEST=scenarios/04-maintenances-cancel-test.js # Run maintenances cancel test
	make k6 TEST=scenarios/05-calendar-test.js # Run calendar test
	make k6 TEST=scenarios/06-ui-maintenance-view-test.js # Run UI maintenance view test

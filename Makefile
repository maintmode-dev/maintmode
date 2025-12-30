# -------------------------------------
# Shared Configuration
# -------------------------------------
# This file contains common configuration variables shared across all Makefiles.
# It is automatically included by Makefile and database.mk.


# GOBIN - Directory for installed Go binaries (goose, golangci-lint, mockgen, etc.)
# Default: ./bin in the project root
GOBIN			?= $(PWD)/bin

# ENV_CONFIG_FILE - Path to environment configuration file
# Used by database.mk to read DB_DRIVER and DB_DSN
# Default: .env.local in the project root
ENV_CONFIG_FILE ?= $(PWD)/.env.local
include ${ENV_CONFIG_FILE}
export

# -------------------------------------
# Database Configuration
# -------------------------------------
# Migration directories for each database type
# These paths are relative to ROOT_DIR (project root)
MIGRATIONS_DIR	?= migrations
DB_DRIVER		?= postgres
DB_DSN			?=

ifndef DB_DSN
DB_DSN=$(shell [ -f $(ENV_CONFIG_FILE) ] && cat $(ENV_CONFIG_FILE) | grep ^DB_DSN | awk '{print $$2}' | sed 's/"//g' || echo "")
endif

# Ensure GOBIN directory exists
$(shell mkdir -p $(GOBIN))


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
.PHONY: bin-deps
bin-deps: bin-deps-build
	GOBIN=$(GOBIN) go install github.com/pressly/goose/v3/cmd/goose@v3.26.0 && \
	GOBIN=$(GOBIN) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.3.0 && \
	GOBIN=$(GOBIN) go install go.uber.org/mock/mockgen@v0.6.0

# -------------------------------------
# Build binary or run app
# -------------------------------------
.PHONY: run
run:
	go run ./cmd/maintmode

.PHONY: build
build:
	$(GOBIN)/goreleaser build --snapshot --single-target --clean --output=$(GOBIN)/maintmode ${args}


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
	go test -p 2 -count 2 ./...
	#cd ./test/via_pkg/ && go test -p 2 -count 2 ./...

# test-cov - Run tests with coverage analysis
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
.PHONY: test-cov
test-cov:
	go test -race -p 2 -count 2 -coverprofile=coverage.tmp -covermode atomic --coverpkg=./internal/... ./...
	@grep -vE "mock|internal/pkg/generated" coverage.tmp > coverage.out
	go tool cover -func=coverage.out | sed 's|github.com/ruko1202/goque||' | sed -E 's/\t+/\t/g' | tee coverage.report

# -------------------------------------
# Linter and formatter
# -------------------------------------

# lint - Run comprehensive linter checks
# Uses golangci-lint with configuration from .golangci.yml
# Checks for code quality, style, bugs, and best practices
.PHONY: lint
lint:
	$(info $(M) running linter...)
	@$(GOBIN)/golangci-lint run

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
	$(GOBIN)/mockgen -typed -destination ./internal/pkg/generated/mocks/mock_storages/storages.go -source ./internal/storages/interface.go


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
	docker compose up -d
	make docker-ps

# docker-down - Stop and remove all database containers
# Stops containers, removes them, and deletes associated volumes
# WARNING: This will delete all data stored in the databases
# Use this when you want a fresh start with empty databases
.PHONY: docker-down
docker-down: ## Stop and remove all database containers with volumes
	$(info $(M) stopping all database containers and removing volumes...)
	docker compose down -v
	make docker-ps

# docker-logs - Stream logs from all database containers
# Shows continuous log output from both PostgreSQL
# Press Ctrl+C to stop following logs
# Useful for debugging connection issues or monitoring queries
.PHONY: docker-logs
docker-logs: ## Show logs from all database containers
	docker compose logs -f

# docker-ps - Show status of database containers
# Displays:
#   - Container name and status (running/stopped)
#   - Ports mapping
#   - Health check status
# Useful for verifying that databases are ready to accept connections
.PHONY: docker-ps
docker-ps: ## Show status of database containers
	docker compose ps -a

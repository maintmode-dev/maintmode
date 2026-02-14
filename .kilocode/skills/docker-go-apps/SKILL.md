---
name: docker-go-apps
description: Docker for Go applications with multi-stage builds, optimization, and .dockerignore. Use when creating Dockerfile for Go applications, optimizing Docker images, configuring docker-compose for Go services, setting up development environments, or deploying Go applications to production.
license: MIT
metadata:
  category: development
  source:
    repository: project-specific
    path: docker-go-apps
---

# Docker for Go Applications

Quick reference for containerizing Go applications with best practices for production deployments.

## Quick Start

### Basic Multi-stage Dockerfile

Multi-stage builds split the build process into multiple stages, significantly reducing the final image size.

```dockerfile
# --- Stage 0: base ---
FROM golang:1.25-alpine AS base
ARG GOBIN=/app/bin
WORKDIR /app
RUN apk add --no-cache make git

COPY Makefile ./
RUN ENVIRONMENT=production > .env.local
RUN make bin-deps-build

# --- Stage 1: build backend (Go) ---
FROM base AS builder
ARG GOBIN=/app/bin
WORKDIR /app

COPY go.mod go.sum ./
RUN make deps

COPY . .
RUN make build args="--id main --output=/app/bin/app"

# --- Stage 2: production image ---
FROM alpine:latest
WORKDIR /app/

RUN apk --no-cache add ca-certificates

COPY --from=builder /app/bin/app .

EXPOSE 8080

CMD ["./app"]
```

### Basic .dockerignore

The `.dockerignore` file excludes unnecessary files from the build context, speeding up the process and reducing image size:

```
# Git
.git
.gitignore

# Documentation
README.md
docs/
*.md

# IDE
.vscode/
.idea/
*.swp
*.swo
*~

# Build artifacts
bin/
dist/
*.exe

# Test files
*_test.go
test/
testdata/

# CI/CD
.github/
.gitlab-ci.yml

# Docker
Dockerfile*
docker-compose*.yml
.dockerignore

# Environment
.env
.env.*
!.env.example

# Logs
*.log
logs/

# Temporary files
tmp/
temp/

# OS files
.DS_Store
Thumbs.db
```

## Detailed References

### [Dockerfile Patterns](references/dockerfile-patterns.md)

Advanced Dockerfile patterns including layer caching optimization, build arguments, and image size reduction techniques.

**When to read:** When optimizing builds, implementing custom build stages, or troubleshooting slow build times.

### [Docker Compose Configuration](references/docker-compose.md)

Complete docker-compose setup with PostgreSQL, health checks, service dependencies, and production configurations.

**When to read:** When setting up local development environment, configuring service dependencies, or implementing health checks.

### [Makefile Integration](references/makefile-integration.md)

Docker commands integrated into Makefile for streamlined development workflow.

**When to read:** When integrating Docker commands into your project's Makefile or automating Docker operations.

## Core Recommendations

1. **Always use multi-stage builds** for production Go application images
2. **Copy go.mod and go.sum separately** for efficient layer caching
3. **Use Alpine Linux** for the final image to minimize size
4. **Configure health checks** for all services
5. **Use .dockerignore** to exclude unnecessary files
6. **Define service dependencies** in docker-compose
7. **Use volumes** for persistent data (databases)
8. **Configure restart policy** for production containers

## Resources

- [Dockerfile Best Practices](https://docs.docker.com/develop/develop-images/dockerfile_best-practices/)
- [Multi-stage builds](https://docs.docker.com/build/building/multi-stage/)
- [Docker Compose Documentation](https://docs.docker.com/compose/)
- [Go Docker Images](https://hub.docker.com/_/golang)

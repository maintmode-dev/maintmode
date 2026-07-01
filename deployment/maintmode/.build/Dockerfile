# Two ways to produce the same runtime image:
#
#   local (default) — self-contained: builds the Go binary inside Docker.
#                     Use for a plain `docker build .` on a dev machine.
#                       docker build -f .../Dockerfile .
#
#   ci              — takes the binary already built by the CI `build-maintmode`
#                     job (which has full git, so goreleaser stamps the real
#                     version + commit) from the build context. No rebuild here —
#                     the GoReleaser-recommended split, and avoids the
#                     version=v0.0.0 / commit=none problem of building without .git.
#                       docker build --target ci -f .../Dockerfile .
#                     Context must provide ./tmp/bin/maintmode (linux/amd64).

# --- Stage 0: base (local build only) ---
FROM golang:1.25-alpine AS base
ARG GOBIN=/app/bin
WORKDIR /app
RUN apk add --no-cache make git

COPY Makefile ./
RUN --mount=type=cache,id=maintmode-go-mod,target=/go/pkg/mod,sharing=locked \
    --mount=type=cache,id=maintmode-go-build,target=/root/.cache/go-build,sharing=locked \
    make bin-deps-build

COPY go.mod go.sum ./
RUN --mount=type=cache,id=maintmode-go-mod,target=/go/pkg/mod,sharing=locked \
    go mod download

# --- Stage 1: build backend (Go) — local build only ---
FROM base AS builder
ARG GOBIN=/app/bin
WORKDIR /app

COPY deployment/maintmode/.build/.goreleaser.yaml ./

COPY web/ web/
COPY cmd/ cmd/
COPY internal/ internal/
COPY docs/ docs/
COPY test/ test/
# NOTE: authz is NOT copied into the builder — `make build` (compilation)
# doesn't read it. Both final targets get authz from the build context in the
# shared `runtime` stage below.

RUN --mount=type=cache,id=maintmode-go-mod,target=/go/pkg/mod,sharing=locked \
    --mount=type=cache,id=maintmode-go-build,target=/root/.cache/go-build,sharing=locked \
    make deps

RUN --mount=type=cache,id=maintmode-go-mod,target=/go/pkg/mod,sharing=locked \
    --mount=type=cache,id=maintmode-go-build,target=/root/.cache/go-build,sharing=locked \
    make build config=.goreleaser.yaml service=maintmode args="--id main --output=/app/bin/maintmode"


# --- Shared runtime base ---
FROM alpine:latest AS runtime
WORKDIR /app/
RUN apk --no-cache add ca-certificates tzdata \
    && adduser -D nonroot
# RBAC model + policy, read via MAINTMODE_AUTHZ_DIR. Comes from the build
# context so both targets share it (the CI target has no builder stage).
COPY deployment/maintmode/authz/ authz/
EXPOSE 8000
EXPOSE 8001
USER nonroot
CMD ["./maintmode"]

# --- Target: ci — copy the prebuilt binary from the build context ---
FROM runtime AS ci
USER root
COPY tmp/bin/maintmode .
# chmod +x too: GitHub's upload/download-artifact does not preserve the
# executable bit, so the artifact may arrive as 0644.
RUN chown nonroot:nonroot maintmode && chmod +x maintmode
USER nonroot

# --- Target: local (default) — use the binary built in the builder stage ---
FROM runtime AS local
USER root
COPY --from=builder /app/bin/maintmode .
RUN chown nonroot:nonroot maintmode
USER nonroot

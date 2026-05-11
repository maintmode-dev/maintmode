# --- Stage 0: base ---
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

# --- Stage 1: build backend (Go) ---
FROM base AS builder
ARG GOBIN=/app/bin
WORKDIR /app

COPY deployment/maintmode/.build/.goreleaser.yaml ./

COPY docs.go ./
COPY web/ web/
COPY cmd/ cmd/
COPY internal/ internal/
COPY docs/ docs/
COPY test/ test/
COPY deployment/maintmode/authz/ deployment/maintmode/authz/

RUN --mount=type=cache,id=maintmode-go-mod,target=/go/pkg/mod,sharing=locked \
    --mount=type=cache,id=maintmode-go-build,target=/root/.cache/go-build,sharing=locked \
    make deps

RUN --mount=type=cache,id=maintmode-go-mod,target=/go/pkg/mod,sharing=locked \
    --mount=type=cache,id=maintmode-go-build,target=/root/.cache/go-build,sharing=locked \
    make build config=.goreleaser.yaml service=maintmode args="--id main --output=/app/bin/maintmode"


# --- Stage 2: production image ---
FROM alpine:latest
WORKDIR /app/

RUN apk --no-cache add ca-certificates tzdata \
    && adduser -D nonroot

COPY --from=builder /app/bin/maintmode .
COPY --from=builder /app/deployment/maintmode/authz/ deployment/maintmode/authz/
RUN chown nonroot:nonroot maintmode

USER nonroot
EXPOSE 8000
EXPOSE 8001

CMD ["./maintmode"]

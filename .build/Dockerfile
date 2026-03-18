# --- Stage 0: base ---
FROM golang:1.25-alpine AS base
ARG GOBIN=/app/bin
WORKDIR /app
RUN apk add --no-cache make git

COPY Makefile ./
RUN echo "ENVIRONMENT=production" > .env.local
# Кэшируем модули и артефакты сборки для бинарных зависимостей
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    make bin-deps-build

COPY go.mod go.sum ./
# Кэшируем скачивание основных зависимостей проекта
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# --- Stage 1: build backend (Go) ---
FROM base AS builder
ARG GOBIN=/app/bin
WORKDIR /app

COPY .goreleaser.yaml ./

COPY docs.go ./
COPY web/ web/
COPY cmd/ cmd/
COPY internal/ internal/
COPY docs/ docs/
COPY test/ test/

# Кэшируем модули и артефакты при выполнении deps и build
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    make deps

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    make build args="--id main --output=/app/bin/maintmode"


# --- Stage 2: production image ---
FROM alpine:latest
WORKDIR /app/

RUN apk --no-cache add ca-certificates tzdata \
    && adduser -D nonroot

COPY --from=builder /app/bin/maintmode .
RUN chown nonroot:nonroot maintmode

USER nonroot
EXPOSE 8080

CMD ["./maintmode"]

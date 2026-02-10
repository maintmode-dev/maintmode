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

COPY .goreleaser.yaml ./

COPY Makefile ./
COPY docs.go ./
COPY web/ web/
COPY cmd/ cmd/
COPY internal/ internal/
COPY docs/ docs/
COPY test/ test/
COPY go.mod go.sum ./

RUN make deps
RUN make build args="--id main --output=/app/bin/maintmode"


# --- Stage 2: production image ---
FROM alpine:latest
WORKDIR /app/

RUN apk --no-cache add ca-certificates

COPY --from=builder /app/bin/maintmode .

EXPOSE 8080

CMD ["./maintmode"]

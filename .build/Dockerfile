# --- Stage 0: base ---
FROM golang:1.25-alpine AS base
WORKDIR /app
ARG GOBIN=/app/bin
RUN apk add --no-cache make git

COPY Makefile ./
RUN make bin-deps-build


# --- Stage 1: build backend (Go) ---
FROM base AS builder

COPY .goreleaser.yaml ./
COPY go.mod go.sum ./

COPY cmd/ cmd/
COPY internal/ internal/

RUN make build


# --- Stage 2: production image ---
FROM alpine:latest
WORKDIR /root/

RUN apk --no-cache add ca-certificates

COPY --from=builder /app/bin/maintmode .

EXPOSE 8080

CMD ["./maintmode"]

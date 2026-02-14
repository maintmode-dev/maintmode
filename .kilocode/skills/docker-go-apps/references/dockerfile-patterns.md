# Dockerfile Patterns

Advanced patterns for building optimized Docker images for Go applications.

## Layer Caching Optimization

To speed up builds, copy files in order from least frequently changed to most frequently changed:

```dockerfile
# First copy go.mod and go.sum - they rarely change
COPY go.mod go.sum ./
RUN go mod download

# Then copy source code
COPY . .

# Build
RUN CGO_ENABLED=0 go build -o /app/main ./cmd/app
```

This approach ensures that dependencies are only re-downloaded when go.mod or go.sum changes, not on every source code change.

## Build Arguments

Use build arguments for configuration:

```dockerfile
ARG GOBIN=/app/bin
ARG ENVIRONMENT=production

ENV ENVIRONMENT=${ENVIRONMENT}
```

Build with custom arguments:

```bash
docker build --build-arg ENVIRONMENT=production -f .build/Dockerfile -t app:prod .
```

## Image Size Optimization

### Use Alpine Linux

Alpine Linux significantly reduces image size:

```dockerfile
# For build stage
FROM golang:1.25-alpine AS builder

# For production
FROM alpine:latest
```

### Install Only Required Dependencies

```dockerfile
RUN apk --no-cache add ca-certificates tzdata
```

### Copy Only Binary Files

```dockerfile
COPY --from=builder /app/bin/app .
```

### Use .dockerignore

A proper `.dockerignore` file excludes unnecessary files from the build context, reducing context size and speeding up builds.

## Useful Commands

### Build Image

```bash
docker build -f .build/Dockerfile -t app:latest .
```

### Build with Arguments

```bash
docker build --build-arg ENVIRONMENT=production -f .build/Dockerfile -t app:prod .
```

### Run Container

```bash
docker run -p 8080:8080 app:latest
```

### View Logs

```bash
docker logs -f app
```

### Enter Container

```bash
docker exec -it app sh
```

### Clean Unused Resources

```bash
docker system prune -a
```

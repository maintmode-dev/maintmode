---
name: zap-logging
description: Structured logging with zap for high-performance logging in Go applications. Use when setting up structured logging with zap, using sugar logger for convenience, adding structured fields, configuring log rotation with lumberjack, integrating xlog for context-based logging, or implementing HTTP logging middleware.
license: MIT
metadata:
  category: development
  source:
    repository: project-specific
    path: zap-logging
---

# Structured Logging with Zap

## Installation

### Install Zap

```bash
go get go.uber.org/zap
go get go.uber.org/zap/zapcore
```

### Project Dependencies

```go
import (
    "go.uber.org/zap"
    "go.uber.org/zap/zapcore"
)
```

## Quick Start

### 1. Create Logger

```go
logger := zap.NewProduction()
defer logger.Sync()

logger.Info("Application started")
```

### 2. Logging with Fields

```go
logger.Info("User created",
    zap.String("user_id", "123"),
    zap.String("username", "john_doe"),
)
```

### 3. Error Logging

```go
err := doSomething()
if err != nil {
    logger.Error("Operation failed",
        zap.Error(err),
        zap.String("operation", "create_user"),
    )
}
```

### 4. Using Sugar Logger

```go
sugar := logger.Sugar()
sugar.Infof("User %s logged in", userID)
sugar.Warnf("Connection timeout: %v", timeout)
```

### 5. Using xlog in Context

```go
import "github.com/ruko1202/maintmode/internal/utils/xlog"

ctx := xlog.WithOperation(ctx, "store.Get")
xlog.FromContext(ctx).Info("Fetching data", zap.String("id", id))
```

## Detailed Guide

For detailed study of each Zap aspect, see corresponding files in [references/](references/):

### [Logger Configuration](references/logger-configuration.md)
- Creating logger for dev and prod environments
- Configuring encoder (JSON, Console)
- Setting up logging levels
- Configuration via Environment

**When to read:** During initial logging setup in the project.

### [Sugar Logger](references/sugar-logger.md)
- Creating and using Sugar logger
- Formatted logging (Infof, Errorf)
- Structured logging with Sugar (Infow)
- When to use Sugar vs regular Logger

**When to read:** For convenient logging without strict performance requirements.

### [Structured Fields](references/structured-fields.md)
- Standard fields (String, Int, Duration, Error)
- Custom fields for business logic
- Logging complex data (Any, Object)
- Arrays and nested structures

**When to read:** When logging with additional data.

### [Log Rotation](references/rotation.md)
- Integration with lumberjack
- Configuring rotation (MaxSize, MaxBackups, MaxAge)
- Configuration for dev and prod
- Managing log file sizes

**When to read:** When setting up production environment.

### [xlog Integration](references/xlog-integration.md)
- xlog package for working with context
- Passing logger through context
- WithOperation, WithRequestID, WithUserID
- Usage in store and service layers

**When to read:** When using architecture with data passed through context.

### [HTTP Middleware](references/middleware.md)
- Middleware for logging HTTP requests
- Capturing response status and duration
- Adding request_id to context
- Integration with echo, gin, chi

**When to read:** When developing HTTP API.

## Best Practices

1. **Use structured logging** - add fields instead of string formatting
2. **Use xlog for context** - pass logger through context
3. **Add operation field** - for tracking execution flow
4. **Log errors with zap.Error()** - for proper display
5. **Use rotation in production** - for log size management
6. **Separate dev and prod configurations** - for development convenience
7. **Add request_id** - for request tracing
8. **Use sugar for simple cases** - for convenient formatting

## Basic Field Types

```go
// String
zap.String("key", "value")

// Numeric
zap.Int("count", 42)
zap.Duration("latency", time.Millisecond*150)

// Boolean
zap.Bool("success", true)

// Errors
zap.Error(err)

// Any type
zap.Any("data", complexObject)
```

## Resources

- [Zap Documentation](https://github.com/uber-go/zap)
- [Zap Best Practices](https://github.com/uber-go/zap/blob/master/FAQ.md)
- [Lumberjack Documentation](https://github.com/natefinch/lumberjack)

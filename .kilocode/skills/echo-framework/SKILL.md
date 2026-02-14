---
name: echo-framework
description: High-performance, extensible, minimalist Go web framework. Use this skill when working with Echo web framework, creating RESTful APIs, handling HTTP requests/responses, setting up middleware, routing, data binding, error handling, or migrating between Echo v4 and v5. Covers both Echo v4 (supported until 2026-12-31) and v5 (latest version) with detailed migration guidance.
license: MIT
metadata:
  category: development
  source:
    repository: project-specific
    path: echo-framework
---

# Echo Framework Skill

## Key Features

- **High Performance**: Optimized HTTP router based on radix tree with zero dynamic memory allocation
- **Minimalist Design**: Simple and intuitive API
- **Flexible Middleware System**: Support for middleware at application, group, or route level
- **Centralized Error Handling**: Unified HTTP error handling mechanism
- **Data Binding**: Support for JSON, XML, and form data
- **Templating**: Support for any template engine
- **HTTP/2 Support**: Built-in HTTP/2 support
- **Automatic TLS**: Let's Encrypt integration

## ⚠️ IMPORTANT: v4 and v5 Version Incompatibility

**Echo v4 and v5 are INCOMPATIBLE with each other.** This is a critical change that requires attention during migration.

### Main Incompatibilities

1. **Context changed from interface to struct**
2. **Logger replaced with standard log/slog**
3. **Most function signatures changed**
4. **Parameters reordered in some functions**

See details in [references/migration-v4-v5.md](references/migration-v4-v5.md)

## Echo Versions

### Supported Versions

- **Echo v5** — latest major version (current as of 2026-01-18)
  - Critical issues with semantic versioning violations will be fixed until 2026-03-31
  - For production, recommended to wait until 2026-03-31 before upgrading
- **Echo v4** — supported with security updates and bug fixes until 2026-12-31

## Installation

### Echo v5

```bash
go get github.com/labstack/echo/v5
```

### Echo v4

```bash
go get github.com/labstack/echo/v4
```

## Quick Start

### Echo v5

```go
package main

import (
    "github.com/labstack/echo/v5"
    "github.com/labstack/echo/v5/middleware"
    "log/slog"
    "net/http"
)

func main() {
    // Create Echo instance
    e := echo.New()

    // Middleware
    e.Use(middleware.RequestLogger())
    e.Use(middleware.Recover())

    // Routes
    e.GET("/", hello)

    // Start server
    if err := e.Start(":8080"); err != nil {
        slog.Error("failed to start server", "error", err)
    }
}

// Handler - IMPORTANT: uses *echo.Context
func hello(c *echo.Context) error {
    return c.String(http.StatusOK, "Hello, World!")
}
```

### Echo v4

```go
package main

import (
    "github.com/labstack/echo/v4"
    "github.com/labstack/echo/v4/middleware"
    "net/http"
)

func main() {
    // Create Echo instance
    e := echo.New()

    // Middleware
    e.Use(middleware.Logger())
    e.Use(middleware.Recover())

    // Routes
    e.GET("/", hello)

    // Start server
    e.Logger.Fatal(e.Start(":8080"))
}

// Handler - uses echo.Context (without pointer)
func hello(c echo.Context) error {
    return c.String(http.StatusOK, "Hello, World!")
}
```

## Main Components

For detailed study of each component, see corresponding files in [references/](references/):

### 1. Routing
Echo uses radix tree-based router for fast route matching.

**Topics:**
- Basic routes (GET, POST, PUT, DELETE)
- Path parameters (:id, wildcard)
- Route groups
- Route naming
- URI generation

**Details:** [references/routing.md](references/routing.md)

### 2. Context
Context represents the context of current HTTP request. In v5, Context changed from interface to struct.

**Topics:**
- Critical changes v4 → v5
- Context methods (common, v5-specific, v4-specific)
- Request and response access
- Working with parameters, cookies, headers

**Details:** [references/context.md](references/context.md)

### 3. Middleware
Middleware are functions that execute before or after request handler.

**Topics:**
- Creating custom middleware
- Built-in middleware (Logger, Recover, CORS, Gzip, BasicAuth, JWT, Rate Limiter)

**Details:** [references/middleware.md](references/middleware.md)

### 4. Binding
Echo provides several ways to bind data from HTTP requests.

**Topics:**
- Struct Tag Binding
- Data source tags (query, param, header, json, xml, form)
- Direct Source Binding
- Fluent Binding
- Generic Parameter Extraction (v5)

**Details:** [references/binding.md](references/binding.md)

### 5. Error Handling
Echo supports centralized error handling.

**Topics:**
- Returning errors
- Custom error handler
- Differences v4 vs v5

**Details:** [references/error-handling.md](references/error-handling.md)

### 6. Server Configuration
Server configuration and startup.

**Topics:**
- Simple startup
- StartConfig (v5)
- TLS configuration
- Graceful shutdown

**Details:** [references/server-config.md](references/server-config.md)

### 7. Logger
Request and error logging.

**Topics:**
- log/slog in v5
- Custom Logger in v4

**Details:** [references/logger.md](references/logger.md)

### 8. Static Files and Templates
Serving static files and rendering templates.

**Topics:**
- Static files (Static, StaticFS, File, FileFS)
- Template rendering (Renderer interface)

**Details:** [references/static-templates.md](references/static-templates.md)

## Migration v4 → v5

**⚠️ Critically important:** Echo v4 and v5 are incompatible. Migration requires significant code changes.

**Main changes:**
- Context: interface → struct (*echo.Context)
- Logger: custom interface → log/slog
- HTTPErrorHandler: parameters reordered
- Middleware: Logger() → RequestLogger()
- Path params: ParamNames()/ParamValues() → PathValues()

**Complete guide:** [references/migration-v4-v5.md](references/migration-v4-v5.md)

## Best Practices

**Key recommendations:**
1. Use route groups
2. Centralized error handling
3. Input data validation
4. Middleware for common tasks
5. Security with binding (DTO → Entity)
6. Context for passing data between middleware
7. Graceful shutdown
8. Proper project structure
9. Configuration via Config
10. Testing with echotest

**Details:** [references/best-practices.md](references/best-practices.md)

## Official Resources

- **Official website**: https://echo.labstack.com
- **Documentation**: https://echo.labstack.com/docs
- **GitHub repository**: https://github.com/labstack/echo
- **GitHub Discussions**: https://github.com/labstack/echo/discussions

## Official Middleware Repositories

| Repository | Description |
|-------------|----------|
| github.com/labstack/echo-jwt | JWT middleware |
| github.com/labstack/echo-contrib | casbin, gorilla/sessions, jaegertracing, prometheus, pprof, zipkin |

## Third-party Libraries

| Repository | Description |
|-------------|----------|
| deepmap/oapi-codegen | OpenAPI code generator |
| github.com/swaggo/echo-swagger | Swagger 2.0 documentation |
| github.com/ziflex/lecho | Zerolog logger wrapper |
| github.com/brpaz/echozap | Zap logger wrapper |
| github.com/samber/slog-echo | slog logger wrapper |

## Go Version Requirements

- **Echo v4**: Go 1.24.0+
- **Echo v5**: Go 1.25.0+

## License

MIT License

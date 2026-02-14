# Project Structure

Standard Go project layout and organization best practices.

## Standard Structure

```
project/
├── cmd/
│   └── appname/
│       └── main.go
├── internal/
│   ├── package1/
│   └── package2/
├── pkg/
│   └── package3/
├── go.mod
├── go.sum
└── README.md
```

## Directory Purposes

### `cmd/`
Application entry points. Each subdirectory is a separate binary.

```
cmd/
├── api/
│   └── main.go      # API server
├── worker/
│   └── main.go      # Background worker
└── cli/
    └── main.go      # CLI tool
```

### `internal/`
Private application and library code. Cannot be imported by other projects.

```
internal/
├── domain/          # Business logic
├── repository/      # Data access
├── service/         # Application services
└── handler/         # HTTP handlers
```

### `pkg/`
Library code that can be used by external applications. Public API of your project.

```
pkg/
├── client/          # Client libraries
└── models/          # Shared data models
```

### `api/`
API definitions - OpenAPI/Swagger specs, Protocol Buffers, etc.

```
api/
├── openapi.yaml
└── proto/
    └── service.proto
```

### `web/`
Web application specific components - templates, static assets.

```
web/
├── static/
│   ├── css/
│   └── js/
└── templates/
    └── index.html
```

### `scripts/`
Build, installation, analysis scripts.

```
scripts/
├── build.sh
├── deploy.sh
└── test.sh
```

### `test/`
Additional external test apps and test data.

```
test/
├── integration/
└── testdata/
```

## Example Clean Architecture Layout

```
project/
├── cmd/
│   └── api/
│       └── main.go
├── internal/
│   ├── domain/              # Entities and business logic
│   │   ├── user/
│   │   │   ├── entity.go
│   │   │   └── repository.go
│   │   └── order/
│   │       ├── entity.go
│   │       └── service.go
│   ├── repository/          # Data access implementations
│   │   ├── postgres/
│   │   │   ├── user.go
│   │   │   └── order.go
│   │   └── redis/
│   │       └── cache.go
│   ├── handler/             # HTTP handlers
│   │   ├── rest/
│   │   │   ├── user.go
│   │   │   └── order.go
│   │   └── middleware/
│   │       └── auth.go
│   ├── config/              # Configuration
│   │   └── config.go
│   └── app/                 # Application setup
│       └── app.go
├── pkg/
│   └── client/              # Client library
│       └── client.go
├── go.mod
└── go.sum
```

## Best Practices

1. **Keep cmd/ minimal** - Just initialization and wiring
2. **Use internal/ for private code** - Enforce boundaries
3. **Put reusable code in pkg/** - Clear public API
4. **Group by feature, not by type** - Better cohesion
5. **Avoid deep nesting** - Keep it flat when possible
6. **Use meaningful package names** - Describe the functionality
7. **One package per directory** - Go convention
8. **Avoid circular dependencies** - Use interfaces

## Package Organization Patterns

### By Feature (Recommended)
```
internal/
├── user/
│   ├── entity.go
│   ├── repository.go
│   ├── service.go
│   └── handler.go
└── order/
    ├── entity.go
    ├── repository.go
    ├── service.go
    └── handler.go
```

### By Layer (Alternative)
```
internal/
├── entity/
│   ├── user.go
│   └── order.go
├── repository/
│   ├── user.go
│   └── order.go
├── service/
│   ├── user.go
│   └── order.go
└── handler/
    ├── user.go
    └── order.go
```

## Initialization Pattern

```go
// cmd/api/main.go
package main

import (
    "log"
    "github.com/user/project/internal/app"
)

func main() {
    if err := app.Run(); err != nil {
        log.Fatal(err)
    }
}

// internal/app/app.go
package app

func Run() error {
    // Load configuration
    // Initialize dependencies
    // Start server
    return nil
}
```

## Resources

- [Standard Go Project Layout](https://github.com/golang-standards/project-layout)
- [Effective Go](https://go.dev/doc/effective_go)

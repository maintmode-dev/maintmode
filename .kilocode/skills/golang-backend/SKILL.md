---
name: golang-backend
description: Comprehensive backend development with Go covering HTTP APIs, databases, microservices, authentication, and deployment. Use when building REST/JSON APIs, implementing HTTP servers with middleware, working with PostgreSQL (pgx) or Redis, setting up JWT authentication, creating gRPC services, configuring structured logging (Zap/Logrus), implementing connection pooling, setting up Docker containers, or architecting production-ready Go backend services.
license: MIT
metadata:
  category: development
  source:
    repository: project-specific
    path: golang-backend
---

# Golang Backend Developer Skill

## Main Components

See detailed sections below:

### 1. HTTP Server and API
- Standard library net/http
- Working with context and timeouts
- Middleware (logging, recovery, CORS, auth)
- JSON API (parsing, error responses, pagination)
- Web frameworks (Gin, Chi, Echo)

### 2. Databases
- **PostgreSQL** with pgx (connection, queries, transactions, batch)
- **SQL** with database/sql
- **GORM** (ORM, models, CRUD)
- **Redis** (caching, sessions)

### 3. Configuration and Logging
- **Viper** for configuration
- **Environment variables**
- **Logrus** and **Zap** for logging
- **Structured logging** with context

### 4. Authentication and Authorization
- **JWT** tokens
- **Password hashing** (bcrypt)
- Middleware for auth

### 5. Microservices
- **gRPC** (proto, server, client)
- **Service Discovery** with Consul

### 6. Docker and Deployment
- Dockerfile (multi-stage builds)
- docker-compose.yml
- Production deployment

### 7. Development Tools
- **Air** (hot reload)
- **golangci-lint** (linting)
- **Makefile** (task automation)

## Usage Examples

### Basic HTTP Server

```go
package main

import (
    "fmt"
    "net/http"
)

func handler(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, "Hello, World!")
}

func main() {
    http.HandleFunc("/", handler)
    http.ListenAndServe(":8080", nil)
}
```

### Middleware pattern

```go
func loggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        next.ServeHTTP(w, r)
        log.Printf("%s %s %v", r.Method, r.URL.Path, time.Since(start))
    })
}
```

### JSON API

```go
type CreateUserRequest struct {
    Name  string `json:"name"`
    Email string `json:"email"`
}

func createUserHandler(w http.ResponseWriter, r *http.Request) {
    var req CreateUserRequest

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }
    defer r.Body.Close()

    // Create user logic...

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(user)
}
```

### Database connection (pgx)

```go
import "github.com/jackc/pgx/v5/pgxpool"

conn, err := pgxpool.New(context.Background(), "postgres://user:pass@localhost/db")
if err != nil {
    log.Fatal(err)
}
defer conn.Close()

rows, err := conn.Query(context.Background(), "SELECT * FROM users")
```

### JWT Authentication

```go
import "github.com/golang-jwt/jwt/v5"

func generateToken(userID string) (string, error) {
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
        "user_id": userID,
        "exp":     time.Now().Add(24 * time.Hour).Unix(),
    })

    return token.SignedString([]byte("secret"))
}
```

## Backend Development Checklist

### Architecture
- [ ] Define project structure (pkg/, internal/, cmd/)
- [ ] Set up routing and middleware
- [ ] Implement error handling
- [ ] Add input data validation

### Database
- [ ] Choose driver (pgx, database/sql, GORM)
- [ ] Set up migrations
- [ ] Create models/schemas
- [ ] Implement repository pattern

### Security
- [ ] Implement authentication (JWT, sessions)
- [ ] Add authorization (roles, permissions)
- [ ] Configure CORS
- [ ] Protect against SQL injection
- [ ] Hash passwords (bcrypt)

### Configuration and Logging
- [ ] Set up configuration (Viper, env vars)
- [ ] Add structured logging (Zap, Logrus)
- [ ] Configure logging levels for dev/prod

### Performance
- [ ] Set up connection pooling
- [ ] Add caching (Redis)
- [ ] Implement pagination
- [ ] Optimize database queries

### Testing
- [ ] Write unit tests
- [ ] Add integration tests
- [ ] Set up test coverage

### Deployment
- [ ] Create Dockerfile
- [ ] Set up docker-compose
- [ ] Add health checks
- [ ] Configure graceful shutdown

### Monitoring
- [ ] Add metrics (Prometheus)
- [ ] Set up production logging
- [ ] Implement tracing (Jaeger, OpenTelemetry)

## Additional Resources

### Documentation
- **Go Documentation**: https://golang.org/doc/
- **Effective Go**: https://golang.org/doc/effective_go
- **Go Wiki**: https://github.com/golang/go/wiki

### Frameworks
- **Gin**: https://gin-gonic.com/
- **Echo**: https://echo.labstack.com/
- **Chi**: https://github.com/go-chi/chi

### Databases
- **pgx**: https://github.com/jackc/pgx
- **GORM**: https://gorm.io/
- **Redis**: https://redis.uptrace.dev/

### Tools
- **Air**: https://github.com/cosmtrek/air
- **golangci-lint**: https://golangci-lint.run/
- **Viper**: https://github.com/spf13/viper

### Patterns and Practices
- **Go Patterns**: https://github.com/tmrts/go-patterns
- **Go Best Practices**: https://github.com/golang-standards/project-layout
- **Awesome Go**: https://awesome-go.com/

---
name: golang-qa
description: Comprehensive testing and QA for Go applications. Use when writing unit tests with table-driven patterns, creating test suites, mocking dependencies (interfaces, HTTP clients, databases), testing HTTP handlers and APIs, setting up integration tests with testcontainers, implementing BDD tests (Ginkgo/Gomega), running load tests (k6, Vegeta), generating test coverage reports, or establishing test infrastructure and CI/CD testing pipelines.
license: MIT
metadata:
  category: development
  source:
    repository: project-specific
    path: golang-qa
---

# Golang QA and Testing

Comprehensive guide to testing Go applications with modern patterns and tools.

## Quick Start

### Basic Test

```go
func TestAdd(t *testing.T) {
    result := Add(2, 3)
    expected := 5

    if result != expected {
        t.Errorf("Add(2, 3) = %d; want %d", result, expected)
    }
}
```

### Table-Driven Test

```go
func TestAdd(t *testing.T) {
    tests := []struct {
        name     string
        a, b     int
        expected int
    }{
        {"positive numbers", 2, 3, 5},
        {"negative numbers", -2, -3, -5},
        {"zero", 0, 0, 0},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := Add(tt.a, tt.b)
            if result != tt.expected {
                t.Errorf("Add(%d, %d) = %d; want %d", tt.a, tt.b, result, tt.expected)
            }
        })
    }
}
```

## Detailed References

### [Table-Driven Tests](references/table-driven-tests.md)

Complete guide to table-driven testing patterns including parallel execution, complex test cases, and best practices.

**When to read:** When writing comprehensive test suites, testing multiple scenarios, or implementing parallel test execution.

### [Mocking Patterns](references/mocking-patterns.md)

Mocking strategies using testify/mock, interface design for testability, and dependency injection patterns.

**When to read:** When testing code with external dependencies, creating testable interfaces, or mocking HTTP clients and databases.

### [Database Testing](references/database-testing.md)

Integration testing with testcontainers, PostgreSQL testing patterns, transaction handling, and database test isolation.

**When to read:** When setting up database integration tests, using testcontainers, or testing repository layers.

### [HTTP Testing](references/http-testing.md)

Testing HTTP handlers, API endpoints, middleware, and using httptest for request/response testing.

**When to read:** When testing HTTP handlers, REST APIs, or web service endpoints.

## Running Tests

```bash
# Run all tests
go test ./...

# Run specific test
go test -run TestAdd

# Run with coverage
go test -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run with race detector
go test -race ./...

# Run benchmarks
go test -bench=. -benchmem
```

## Best Practices

1. **Use table-driven tests** - Test multiple cases efficiently
2. **Name tests descriptively** - Explain what's being tested
3. **Use subtests with t.Run()** - Better organization
4. **Test behavior, not implementation** - Focus on outcomes
5. **Mock external dependencies** - Faster, more reliable tests
6. **Use testcontainers for integration tests** - Real database testing
7. **Run tests with -race flag** - Detect race conditions
8. **Aim for high coverage** - But focus on critical paths
9. **Use t.Parallel() for independent tests** - Faster test execution
10. **Clean up resources in tests** - Use t.Cleanup() or defer

## Resources

### Testing Frameworks
- [Go testing package](https://pkg.go.dev/testing)
- [Testify](https://github.com/stretchr/testify)
- [Ginkgo](https://onsi.github.io/ginkgo/)
- [Gomega](https://onsi.github.io/gomega/)

### Test Infrastructure
- [Testcontainers](https://golang.testcontainers.org/)
- [httptest](https://pkg.go.dev/net/http/httptest)

### Load Testing
- [k6](https://k6.io/)
- [Vegeta](https://github.com/tsenart/vegeta)

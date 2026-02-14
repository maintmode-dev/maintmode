---
name: golang-coder
description: Best practices for writing idiomatic Go code following Effective Go principles. Use when writing new Go code, refactoring existing code, reviewing Go implementations, explaining Go idioms and patterns, or writing unit tests. Covers naming conventions, error handling, interfaces, concurrency patterns, testing strategies, and project structure.
license: MIT
metadata:
  category: development
  source:
    repository: project-specific
    path: golang-coder
---

# Golang Coder

Best practices and idioms for writing clean, idiomatic Go code.

## Go Core Principles

### Language Philosophy
- **Less is more** - Simplicity and clarity
- **Explicit is better than implicit** - Favor explicitness
- **Composition over inheritance** - Build with composition
- **Do one thing well** - Focus on single responsibility

### Key Idioms

#### Variable Declaration
```go
// Short declaration (only inside functions)
x := 42

// Full declaration
var x int = 42

// Multiple variables
x, y := 1, 2

// Idiom for checking with ok
if v, ok := m["key"]; ok {
    // v contains value, key exists
}
```

#### Defer
```go
// Defer executes in LIFO order on function exit
func process() error {
    f, err := os.Open("file.txt")
    if err != nil {
        return err
    }
    defer f.Close() // Will close on exit

    // ... work with file
}
```

#### Make vs New
```go
// make - for slices, maps, channels (initializes memory)
s := make([]int, 0, 10)
m := make(map[string]int)
ch := make(chan int)

// new - returns pointer to zero value
p := new(int) // *int with value 0
```

## Detailed References

### [Naming Conventions](references/naming-conventions.md)

Complete guide to Go naming conventions including visibility, package names, interfaces, getters/setters, and constants.

**When to read:** When naming new types, functions, or packages, or when reviewing code for style consistency.

### [Error Handling](references/error-handling.md)

Comprehensive error handling patterns including wrapping, typed errors, and panic/recover usage.

**When to read:** When implementing error handling, creating custom errors, or debugging error-related issues.

### [Interfaces and Types](references/interfaces-types.md)

Working with interfaces, type assertions, type switches, slices, maps, and embedding.

**When to read:** When designing interfaces, working with type conversions, or implementing composition patterns.

### [Concurrency](references/concurrency.md)

Goroutines, channels, worker pools, select statements, and sync primitives.

**When to read:** When implementing concurrent code, debugging race conditions, or optimizing parallel processing.

### [Testing](references/testing.md)

Testing strategies including table-driven tests, benchmarks, examples, and mocking.

**When to read:** When writing tests, benchmarks, or setting up test infrastructure.

### [Project Structure](references/project-structure.md)

Standard project layout and organization best practices.

**When to read:** When starting a new project or organizing an existing codebase.

## Quick Reference

### Formatting Tools

```bash
gofmt -w .       # Format all files
go vet ./...     # Check for common errors
goimports -w .   # Format + sort imports
```

### Code Quality Checklist

Before completing your work, verify:
- [ ] All exported names start with capital letter
- [ ] Errors are handled everywhere they can occur
- [ ] defer is used for resource cleanup
- [ ] Code is formatted (gofmt)
- [ ] No race conditions (go test -race)
- [ ] Tests cover main scenarios
- [ ] Documentation for exported functions/types
- [ ] No unused variables and imports

## Resources

- [Effective Go](https://go.dev/doc/effective_go)
- [Go Language Specification](https://golang.org/ref/spec)
- [Go Standard Library](https://golang.org/pkg/)

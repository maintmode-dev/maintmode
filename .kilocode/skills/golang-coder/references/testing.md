# Testing

Testing strategies and patterns in Go.

## Table-Driven Tests

```go
func TestAdd(t *testing.T) {
    tests := []struct {
        name     string
        a, b     int
        expected int
    }{
        {"positive", 2, 3, 5},
        {"negative", -2, -3, -5},
        {"zero", 0, 0, 0},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := Add(tt.a, tt.b)
            if result != tt.expected {
                t.Errorf("Add(%d, %d) = %d; want %d",
                    tt.a, tt.b, result, tt.expected)
            }
        })
    }
}
```

## Benchmarks

```go
func BenchmarkAdd(b *testing.B) {
    for i := 0; i < b.N; i++ {
        Add(2, 3)
    }
}
```

Run benchmarks:
```bash
go test -bench=. -benchmem
```

## Examples

```go
// ExampleAdd demonstrates the Add function.
func ExampleAdd() {
    result := Add(2, 3)
    fmt.Println(result)
    // Output: 5
}
```

## Mock Interfaces

```go
// Mock implementation of interface
type mockReader struct {
    data []byte
    err  error
}

func (m *mockReader) Read(p []byte) (n int, err error) {
    if m.err != nil {
        return 0, m.err
    }
    copy(p, m.data)
    return len(m.data), nil
}

func TestProcess(t *testing.T) {
    mock := &mockReader{data: []byte("test")}
    err := process(mock)
    if err != nil {
        t.Fatal(err)
    }
}
```

## Test Helpers

```go
func TestMain(m *testing.M) {
    // Setup
    setup()

    // Run tests
    code := m.Run()

    // Teardown
    teardown()

    os.Exit(code)
}

func setup() {
    // Initialize test resources
}

func teardown() {
    // Cleanup test resources
}
```

## Testing HTTP Handlers

```go
func TestHandler(t *testing.T) {
    req := httptest.NewRequest("GET", "/users", nil)
    rec := httptest.NewRecorder()

    handler(rec, req)

    if rec.Code != http.StatusOK {
        t.Errorf("expected status 200, got %d", rec.Code)
    }
}
```

## Best Practices

1. **Use table-driven tests** - Test multiple cases efficiently
2. **Name tests descriptively** - Explain what's being tested
3. **Test behavior, not implementation** - Focus on outcomes
4. **Use t.Helper() for test helpers** - Better error messages
5. **Avoid global state in tests** - Tests should be independent
6. **Use subtests with t.Run()** - Better organization and parallel execution
7. **Write examples for documentation** - Executable documentation
8. **Use mocks for external dependencies** - Faster, more reliable tests
9. **Run tests with -race flag** - Detect race conditions
10. **Aim for high coverage** - But don't chase 100%

## Running Tests

```bash
# Run all tests
go test ./...

# Run specific test
go test -run TestAdd

# Run with race detector
go test -race ./...

# Run with coverage
go test -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

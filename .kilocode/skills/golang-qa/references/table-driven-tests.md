# Table-Driven Tests

Complete guide to table-driven testing patterns in Go.

## Basic Pattern

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
        {"one negative", -1, 2, 1},
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

## Parallel Execution

```go
func TestAdd(t *testing.T) {
    t.Parallel() // Run test in parallel

    tests := []struct {
        name     string
        a, b     int
        expected int
    }{
        {"positive", 2, 3, 5},
        {"negative", -2, -3, -5},
    }

    for _, tt := range tests {
        tt := tt // Important! Capture range variable
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel() // Subtests also parallel
            result := Add(tt.a, tt.b)
            if result != tt.expected {
                t.Errorf("got %d, want %d", result, tt.expected)
            }
        })
    }
}
```

**Important**: Always capture range variable when using t.Parallel() in subtests.

## Complex Test Cases

### Testing Errors

```go
func TestDivide(t *testing.T) {
    tests := []struct {
        name      string
        a, b      int
        expected  int
        expectErr bool
    }{
        {"valid division", 10, 2, 5, false},
        {"divide by zero", 10, 0, 0, true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := Divide(tt.a, tt.b)

            if tt.expectErr {
                if err == nil {
                    t.Error("expected error, got nil")
                }
                return
            }

            if err != nil {
                t.Errorf("unexpected error: %v", err)
            }

            if result != tt.expected {
                t.Errorf("got %d, want %d", result, tt.expected)
            }
        })
    }
}
```

### Testing with Slices

```go
func TestFilter(t *testing.T) {
    tests := []struct {
        name     string
        input    []int
        filter   func(int) bool
        expected []int
    }{
        {
            name:     "filter evens",
            input:    []int{1, 2, 3, 4, 5},
            filter:   func(n int) bool { return n%2 == 0 },
            expected: []int{2, 4},
        },
        {
            name:     "filter odds",
            input:    []int{1, 2, 3, 4, 5},
            filter:   func(n int) bool { return n%2 != 0 },
            expected: []int{1, 3, 5},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := Filter(tt.input, tt.filter)
            if !reflect.DeepEqual(result, tt.expected) {
                t.Errorf("got %v, want %v", result, tt.expected)
            }
        })
    }
}
```

## Using Testify Assertions

```go
import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestUser(t *testing.T) {
    tests := []struct {
        name   string
        userID int
        expect func(t *testing.T, user *User, err error)
    }{
        {
            name:   "valid user",
            userID: 123,
            expect: func(t *testing.T, user *User, err error) {
                require.NoError(t, err)
                require.NotNil(t, user)
                assert.Equal(t, 123, user.ID)
                assert.Equal(t, "John", user.Name)
            },
        },
        {
            name:   "user not found",
            userID: 999,
            expect: func(t *testing.T, user *User, err error) {
                assert.Error(t, err)
                assert.Nil(t, user)
            },
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            user, err := GetUser(tt.userID)
            tt.expect(t, user, err)
        })
    }
}
```

## Best Practices

1. **Use descriptive test names** - Explain what's being tested
2. **Capture range variables** - Always use `tt := tt` before t.Parallel()
3. **Use t.Run for subtests** - Better organization and output
4. **Test one thing per subtest** - Easier to debug
5. **Use helper functions** - Extract common test logic
6. **Test edge cases** - Include boundary conditions
7. **Use require for critical checks** - Stop on failure
8. **Use assert for multiple checks** - Continue on failure

## Test Helpers

```go
func assertEqual(t *testing.T, got, want interface{}) {
    t.Helper() // Mark as helper function
    if !reflect.DeepEqual(got, want) {
        t.Errorf("got %v, want %v", got, want)
    }
}

func TestWithHelper(t *testing.T) {
    tests := []struct {
        name     string
        input    int
        expected int
    }{
        {"double 2", 2, 4},
        {"double 5", 5, 10},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := Double(tt.input)
            assertEqual(t, result, tt.expected)
        })
    }
}
```

## Running Table-Driven Tests

```bash
# Run all tests
go test ./...

# Run specific test table
go test -run TestAdd

# Run specific subtest
go test -run TestAdd/positive

# Run in parallel
go test -parallel 4 ./...

# Verbose output
go test -v ./...
```

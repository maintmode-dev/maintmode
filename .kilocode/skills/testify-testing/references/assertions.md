# Assertions: require vs assert

Testify provides two packages for assertions: `require` and `assert`. Understanding when to use each is crucial for writing effective tests.

## Key Differences

| Feature | require | assert |
|---------|---------|--------|
| Behavior on failure | Stops test execution (`t.FailNow()`) | Continues test execution (`t.Fail()`) |
| Use case | Critical assertions | Non-critical checks |
| Test readability | Cleaner (no need for return) | May clutter with continued execution |

## Using require (Recommended Default)

Use `require` for most assertions. When a test fails, it stops immediately, providing clear failure points:

```go
import (
    "testing"
    "github.com/stretchr/testify/require"
)

func TestUserCreation(t *testing.T) {
    user, err := CreateUser("john@example.com")
    require.NoError(t, err) // Stop if creation fails
    require.NotNil(t, user) // Stop if user is nil

    // Safe to access user properties - we know it's not nil
    require.Equal(t, "john@example.com", user.Email)
    require.NotEmpty(t, user.ID)
}
```

## Using assert (Rare Cases)

Use `assert` only when you want to check multiple independent conditions and continue after failures:

```go
import (
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestUserValidation(t *testing.T) {
    user := &User{
        Email: "invalid-email",
        Age:   -5,
        Name:  "",
    }

    // Check all validation errors at once
    assert.False(t, user.IsValidEmail(), "email should be invalid")
    assert.False(t, user.IsValidAge(), "age should be invalid")
    assert.False(t, user.IsValidName(), "name should be invalid")
    // Test continues even if assertions fail
}
```

**Warning:** Be careful with assert - accessing nil values after failed assertions can cause panics.

## Common Assertions

### Equality

```go
// Basic equality
require.Equal(t, expected, actual)
require.Equal(t, 42, result)
require.Equal(t, "hello", str)

// Deep equality for complex types
require.Equal(t, expectedUser, actualUser)

// Not equal
require.NotEqual(t, oldValue, newValue)
```

### Nil Checks

```go
// Nil assertions
require.Nil(t, err)
require.NotNil(t, user)

// Specific for errors (preferred)
require.NoError(t, err)
require.Error(t, err)
```

### Boolean Assertions

```go
require.True(t, isValid)
require.False(t, isDeleted)
```

### String Assertions

```go
require.Empty(t, str)
require.NotEmpty(t, str)

require.Contains(t, "hello world", "world")
require.NotContains(t, "hello", "goodbye")

require.Equal(t, "expected", str) // Exact match
```

### Collection Assertions

```go
// Slice/Array
require.Len(t, users, 5)
require.Empty(t, emptySlice)
require.NotEmpty(t, items)

require.Contains(t, []string{"a", "b", "c"}, "b")
require.ElementsMatch(t, []int{1, 2, 3}, []int{3, 1, 2}) // Same elements, any order

// Map
require.Contains(t, map[string]int{"a": 1, "b": 2}, "a")
```

### Type Assertions

```go
require.IsType(t, &User{}, result)
require.Implements(t, (*io.Reader)(nil), reader)
```

### Numeric Assertions

```go
require.Greater(t, 10, 5)
require.GreaterOrEqual(t, 10, 10)
require.Less(t, 5, 10)
require.LessOrEqual(t, 5, 5)

require.Positive(t, count)
require.Negative(t, delta)
require.Zero(t, balance)
require.NotZero(t, id)

// Float comparison with delta
require.InDelta(t, 3.14, pi, 0.01)
```

## Error Assertions

```go
// Basic error checks
require.NoError(t, err)
require.Error(t, err)

// Check error message
require.EqualError(t, err, "expected error message")
require.ErrorContains(t, err, "partial message")

// Check error type
require.ErrorIs(t, err, ErrNotFound)
require.ErrorAs(t, err, &ValidationError{})
```

## Custom Messages

Add custom messages for better test output:

```go
require.Equal(t, expected, actual, "user ID should match")
require.NoError(t, err, "failed to create maintenance")
require.NotNil(t, result, "GetUser should return a user for valid ID")
```

## MaintMode Project Patterns

Common patterns used in the MaintMode project:

### Database Operations

```go
func TestDatabaseCreate(t *testing.T) {
    err := store.Create(ctx, entity)
    require.NoError(t, err)

    result, err := store.Get(ctx, entity.ID)
    require.NoError(t, err)
    require.NotNil(t, result)
    require.Equal(t, entity, result)
}
```

### Error Cases

```go
func TestInvalidInput(t *testing.T) {
    err := store.Create(ctx, invalidEntity)
    require.Error(t, err)
    require.ErrorContains(t, err, "violates check constraint")

    result, err := store.Get(ctx, invalidEntity.ID)
    require.EqualError(t, err, apperr.ErrMaintNotFound.Error())
    require.Nil(t, result)
}
```

### Collection Validation

```go
func TestList(t *testing.T) {
    maints, truncated, err := store.GetMaints(ctx, filter, limit)
    require.NoError(t, err)
    require.False(t, truncated)
    require.GreaterOrEqual(t, len(maints), 3)
}
```

## Best Practices

1. **Default to require** - Use `require` for almost all assertions
2. **Use assert sparingly** - Only for independent checks where continuing makes sense
3. **Check errors first** - Always assert `NoError` before using returned values
4. **Check nil before access** - Always assert `NotNil` before accessing struct fields
5. **Add context** - Use custom messages for complex assertions
6. **Be specific** - Use the most specific assertion (e.g., `NoError` vs `Nil`)
7. **Avoid assert in critical paths** - Never use `assert` when failure could cause panics

## Anti-Patterns

```go
// BAD: Using assert when value might be nil
user, err := GetUser(id)
assert.NoError(t, err)         // Test continues even if err != nil
assert.Equal(t, "John", user.Name) // PANIC if user is nil!

// GOOD: Using require
user, err := GetUser(id)
require.NoError(t, err)        // Stop if err != nil
require.NotNil(t, user)        // Stop if user is nil
require.Equal(t, "John", user.Name) // Safe - we know user is not nil
```

```go
// BAD: No error check
result := RiskyOperation()
require.Equal(t, expected, result) // What if RiskyOperation failed?

// GOOD: Check error first
result, err := RiskyOperation()
require.NoError(t, err)
require.Equal(t, expected, result)
```

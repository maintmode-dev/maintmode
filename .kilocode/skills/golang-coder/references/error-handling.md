# Error Handling

Comprehensive guide to error handling patterns in Go.

## Basic Pattern

```go
func readFile(path string) ([]byte, error) {
    f, err := os.Open(path)
    if err != nil {
        return nil, err  // always return error
    }
    defer f.Close()

    data, err := io.ReadAll(f)
    if err != nil {
        return nil, err
    }

    return data, nil
}
```

## Error Wrapping

```go
import "fmt"

func processUser(id string) error {
    user, err := getUser(id)
    if err != nil {
        return fmt.Errorf("failed to get user %s: %w", id, err)
    }
    // ...
}
```

## Typed Errors

```go
import "errors"

var (
    ErrNotFound = errors.New("not found")
    ErrInvalid  = errors.New("invalid input")
)

func getUser(id string) (*User, error) {
    if id == "" {
        return nil, ErrInvalid
    }
    // ...
}
```

## Checking Error Types

```go
if errors.Is(err, ErrNotFound) {
    // handle specific error
}

var notFound *NotFoundError
if errors.As(err, &notFound) {
    // handle by type
}
```

## Panic and Recover

```go
// Panic only for truly unrecoverable errors
func mustLoadConfig(path string) *Config {
    cfg, err := loadConfig(path)
    if err != nil {
        panic(fmt.Sprintf("failed to load config: %v", err))
    }
    return cfg
}

// Recover only in defer, as last resort
func safeOperation() (err error) {
    defer func() {
        if r := recover(); r != nil {
            err = fmt.Errorf("panic recovered: %v", r)
        }
    }()
    // ... code that might panic
    return nil
}
```

## Best Practices

1. **Always handle errors** - Don't ignore them
2. **Return errors, don't panic** - Panic only for programmer errors
3. **Wrap errors with context** - Use fmt.Errorf with %w
4. **Use typed errors for specific cases** - Define sentinel errors
5. **Check error types with errors.Is/As** - Don't compare directly
6. **Add context when propagating** - Help debugging with meaningful messages

## Sequential Error Handling

```go
// Good - errors handled sequentially
func process() error {
    if err := step1(); err != nil {
        return err
    }

    if err := step2(); err != nil {
        return err
    }

    return step3()
}
```

## Early Return Pattern

```go
// Good - early return
func process(input string) error {
    if input == "" {
        return errors.New("empty input")
    }

    if len(input) > 100 {
        return errors.New("input too long")
    }

    // main logic
    return nil
}

// Bad - deep nesting
func process(input string) error {
    if input != "" {
        if len(input) <= 100 {
            // main logic
            return nil
        } else {
            return errors.New("input too long")
        }
    } else {
        return errors.New("empty input")
    }
}
```

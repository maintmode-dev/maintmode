# Structured Fields

## Standard field types

### String fields

```go
zap.String("user_id", "123")
zap.ByteString("data", []byte("hello"))
zap.Strings("tags", []string{"tag1", "tag2"})
```

### Numeric fields

```go
// Integers
zap.Int("count", 42)
zap.Int64("big_number", 1234567890)
zap.Uint("unsigned", 100)

// Floating-point numbers
zap.Float64("price", 19.99)
zap.Float32("rating", 4.5)
```

### Boolean fields

```go
zap.Bool("is_active", true)
zap.Bool("verified", false)
```

### Time fields

```go
zap.Time("created_at", time.Now())
zap.Duration("latency", time.Millisecond*150)
```

### Errors

```go
zap.Error(err)
zap.NamedError("db_error", dbErr)
```

## Custom fields for business logic

### UUID

```go
zap.String("id", uuid.New().String())
zap.String("maintenance_id", maint.ID.String())
```

### Enum types

```go
zap.String("status", string(entity.StatusActive))
zap.String("type", string(resource.TypeServer))
```

### JSON data

```go
jsonData, _ := json.Marshal(payload)
zap.String("payload", string(jsonData))
```

## Logging complex data

### Any - the universal type

```go
// Logging a struct
user := &User{ID: "123", Name: "John"}
logger.Info("User data", zap.Any("user", user))

// Logging a map
metadata := map[string]interface{}{
    "key1": "value1",
    "key2": 42,
}
logger.Info("Metadata", zap.Any("metadata", metadata))
```

### Object - custom serialization

```go
// Implementing ObjectMarshaler
type User struct {
    ID   string
    Name string
}

func (u *User) MarshalLogObject(enc zapcore.ObjectEncoder) error {
    enc.AddString("id", u.ID)
    enc.AddString("name", u.Name)
    return nil
}

// Usage
user := &User{ID: "123", Name: "John"}
logger.Info("User created", zap.Object("user", user))
```

## Arrays and nested structures

### Array fields

```go
// Array of strings
zap.Strings("tags", []string{"tag1", "tag2", "tag3"})

// Array of numbers
zap.Ints("ids", []int{1, 2, 3})

// Custom array
type UserArray []User

func (ua UserArray) MarshalLogArray(enc zapcore.ArrayEncoder) error {
    for _, u := range ua {
        enc.AppendObject(&u)
    }
    return nil
}

logger.Info("Users", zap.Array("users", UserArray{user1, user2}))
```

## Standard fields for HTTP

```go
logger.Info("HTTP request",
    zap.String("method", r.Method),
    zap.String("path", r.URL.Path),
    zap.String("remote_addr", r.RemoteAddr),
    zap.String("user_agent", r.UserAgent()),
    zap.Int("status_code", statusCode),
    zap.Duration("duration", time.Since(start)),
)
```

## Standard fields for operations

```go
// Request ID
zap.String("request_id", requestID)

// Operation name
zap.String("operation", "store.Maintenances.Get")

// User ID
zap.String("user_id", userID)

// Duration
zap.Duration("duration", time.Since(start))

// Status code
zap.Int("status_code", statusCode)

// Error
zap.Error(err)

// Stack trace
zap.Stack("stack")
```

## Field performance

| Field type | Allocation | Recommendation |
|----------|-----------|--------------|
| String | Zero | ✅ Use it |
| Int | Zero | ✅ Use it |
| Any | Allocates | ⚠️ For prototypes |
| Error | Zero | ✅ Use it |
| Object | Zero* | ✅ For structs |

*Zero if ObjectMarshaler is implemented efficiently

## Best Practices

1. **Use typed fields** instead of Any whenever possible
2. **Add context** - operation, request_id, user_id
3. **Measure time** - use Duration for performance tracking
4. **Structure errors** - use Error() so they are formatted correctly
5. **Avoid nesting** - prefer a flat field structure

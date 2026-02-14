# Structured Fields

## Стандартные типы полей

### Строковые поля

```go
zap.String("user_id", "123")
zap.ByteString("data", []byte("hello"))
zap.Strings("tags", []string{"tag1", "tag2"})
```

### Числовые поля

```go
// Целые числа
zap.Int("count", 42)
zap.Int64("big_number", 1234567890)
zap.Uint("unsigned", 100)

// Числа с плавающей точкой
zap.Float64("price", 19.99)
zap.Float32("rating", 4.5)
```

### Булевы поля

```go
zap.Bool("is_active", true)
zap.Bool("verified", false)
```

### Временные поля

```go
zap.Time("created_at", time.Now())
zap.Duration("latency", time.Millisecond*150)
```

### Ошибки

```go
zap.Error(err)
zap.NamedError("db_error", dbErr)
```

## Кастомные поля для бизнес-логики

### UUID

```go
zap.String("id", uuid.New().String())
zap.String("maintenance_id", maint.ID.String())
```

### Enum типы

```go
zap.String("status", string(entity.StatusActive))
zap.String("type", string(resource.TypeServer))
```

### JSON данные

```go
jsonData, _ := json.Marshal(payload)
zap.String("payload", string(jsonData))
```

## Логирование сложных данных

### Any - универсальный тип

```go
// Логирование структуры
user := &User{ID: "123", Name: "John"}
logger.Info("User data", zap.Any("user", user))

// Логирование map
metadata := map[string]interface{}{
    "key1": "value1",
    "key2": 42,
}
logger.Info("Metadata", zap.Any("metadata", metadata))
```

### Object - кастомная сериализация

```go
// Реализация ObjectMarshaler
type User struct {
    ID   string
    Name string
}

func (u *User) MarshalLogObject(enc zapcore.ObjectEncoder) error {
    enc.AddString("id", u.ID)
    enc.AddString("name", u.Name)
    return nil
}

// Использование
user := &User{ID: "123", Name: "John"}
logger.Info("User created", zap.Object("user", user))
```

## Массивы и вложенные структуры

### Array поля

```go
// Массив строк
zap.Strings("tags", []string{"tag1", "tag2", "tag3"})

// Массив чисел
zap.Ints("ids", []int{1, 2, 3})

// Кастомный массив
type UserArray []User

func (ua UserArray) MarshalLogArray(enc zapcore.ArrayEncoder) error {
    for _, u := range ua {
        enc.AppendObject(&u)
    }
    return nil
}

logger.Info("Users", zap.Array("users", UserArray{user1, user2}))
```

## Стандартные поля для HTTP

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

## Стандартные поля для операций

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

## Производительность полей

| Тип поля | Allocation | Рекомендация |
|----------|-----------|--------------|
| String | Zero | ✅ Используйте |
| Int | Zero | ✅ Используйте |
| Any | Allocates | ⚠️ Для прототипов |
| Error | Zero | ✅ Используйте |
| Object | Zero* | ✅ Для структур |

*Zero если ObjectMarshaler реализован эффективно

## Best Practices

1. **Используйте типизированные поля** вместо Any когда возможно
2. **Добавляйте контекст** - operation, request_id, user_id
3. **Измеряйте время** - используйте Duration для performance tracking
4. **Структурируйте ошибки** - используйте Error() для корректного форматирования
5. **Избегайте вложенности** - предпочитайте плоскую структуру полей

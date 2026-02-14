# Middleware

Middleware — это функции, которые выполняются до или после обработчика запроса.

## Создание кастомного middleware

### Echo v5

```go
// Middleware использует *echo.Context
func customMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c *echo.Context) error {
        // Код до выполнения обработчика
        c.Logger().Info("request started")

        // Вызов следующего обработчика
        err := next(c)

        // Код после выполнения обработчика
        c.Logger().Info("request completed")

        return err
    }
}

// Использование
e.Use(customMiddleware)
```

### Echo v4

```go
// Middleware использует echo.Context
func customMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        // Код до выполнения обработчика
        c.Logger().Info("request started")

        // Вызов следующего обработчика
        err := next(c)

        // Код после выполнения обработчика
        c.Logger().Info("request completed")

        return err
    }
}

// Использование
e.Use(customMiddleware)
```

## Встроенные middleware

### Logger

```go
// v5
e.Use(middleware.RequestLogger())

// v4
e.Use(middleware.Logger())
```

### Recover

```go
// v5 и v4 (одинаково)
e.Use(middleware.Recover())
```

### CORS

```go
// v5 - принимает origins
e.Use(middleware.CORS("https://example.com", "https://api.example.com"))

// v4
e.Use(middleware.CORS())
```

### Body Limit

```go
// v5 - принимает байты
e.Use(middleware.BodyLimit(10 * 1024 * 1024)) // 10MB

// v4
e.Use(middleware.BodyLimit("10M"))
```

### Gzip

```go
// v5 и v4 (одинаково)
e.Use(middleware.Gzip())
```

### Basic Auth

```go
// v5
e.Use(middleware.BasicAuth(func(c *echo.Context, username, password string) (bool, error) {
    return username == "admin" && password == "secret", nil
}))

// v4
e.Use(middleware.BasicAuth(func(username, password string, c echo.Context) (bool, error) {
    return username == "admin" && password == "secret", nil
}))
```

### JWT

```go
// v5
e.Use(middleware.JWT([]byte("secret")))

// v4
e.Use(middleware.JWT([]byte("secret")))
```

### Rate Limiter

```go
// v5
e.Use(middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(100)))

// v4
e.Use(middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(100)))
```

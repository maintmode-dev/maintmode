# Лучшие практики

## 1. Используйте группы маршрутов

```go
// API v1
v1 := e.Group("/api/v1")
v1.GET("/users", getUsers)
v1.POST("/users", createUser)

// Admin routes
admin := e.Group("/admin", adminAuthMiddleware)
admin.GET("/dashboard", dashboard)
```

## 2. Централизованная обработка ошибок

```go
e.HTTPErrorHandler = func(c *echo.Context, err error) {
    // Логирование
    c.Logger().Error(err)

    // Отправка клиенту
    var code int
    var message string

    if he, ok := err.(*echo.HTTPError); ok {
        code = he.Code
        message = he.Message
    } else {
        code = http.StatusInternalServerError
        message = "Internal server error"
    }

    c.JSON(code, map[string]string{"error": message})
}
```

## 3. Валидация входных данных

```go
type CreateUserRequest struct {
    Name  string `json:"name" validate:"required,min=3,max=100"`
    Email string `json:"email" validate:"required,email"`
    Age   int    `json:"age" validate:"gte=0,lte=150"`
}

func createUser(c *echo.Context) error {
    req := new(CreateUserRequest)
    if err := c.Bind(req); err != nil {
        return err
    }

    if err := c.Validate(req); err != nil {
        return echo.NewHTTPError(http.StatusBadRequest, err.Error())
    }

    // ...
}
```

## 4. Используйте middleware для общих задач

```go
// Логирование
e.Use(middleware.RequestLogger())

// Восстановление от паник
e.Use(middleware.Recover())

// CORS
e.Use(middleware.CORS())

// Rate limiting
e.Use(middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(100)))
```

## 5. Безопасность при binding

```go
// DTO для входных данных
type UserDTO struct {
    Name  string `json:"name"`
    Email string `json:"email"`
}

// Бизнес-структура
type User struct {
    ID     string
    Name   string
    Email  string
    IsAdmin bool // Не должно биндиться!
}

func createUser(c *echo.Context) error {
    dto := new(UserDTO)
    if err := c.Bind(dto); err != nil {
        return err
    }

    // Явный маппинг для безопасности
    user := User{
        Name:  dto.Name,
        Email: dto.Email,
        IsAdmin: false, // Всегда false при создании
    }

    // ...
}
```

## 6. Используйте context для передачи данных между middleware

```go
// В middleware
func authMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c *echo.Context) error {
        user := authenticate(c)
        c.Set("user", user)
        return next(c)
    }
}

// В handler
func handler(c *echo.Context) error {
    // v5 - типобезопасное получение
    user, err := echo.ContextGet[*User](c, "user")
    if err != nil {
        return err
    }

    // v4
    user := c.Get("user").(*User)
}
```

## 7. Graceful shutdown

```go
// v5
ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer cancel()

sc := echo.StartConfig{
    Address:          ":8080",
    GracefulTimeout: 10 * time.Second,
}

go func() {
    if err := sc.Start(ctx, e); err != nil {
        log.Fatal(err)
    }
}()

<-ctx.Done()
```

## 8. Структура проекта

```
cmd/
  app/
    main.go
internal/
  handlers/
    user.go
    auth.go
  middleware/
    auth.go
    logging.go
  models/
    user.go
  services/
    user_service.go
  repository/
    user_repository.go
```

## 9. Конфигурация

```go
// v5 - используйте Config
config := echo.Config{
    Logger:   slog.New(slog.NewJSONHandler(os.Stdout, nil)),
    Binder:   &CustomBinder{},
    Renderer: &TemplateRenderer{},
}

e := echo.NewWithConfig(config)
```

## 10. Тестирование

```go
import "github.com/labstack/echo/v5/echotest"

func TestHandler(t *testing.T) {
    e := echo.New()
    req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
    rec := httptest.NewRecorder()
    c := e.NewContext(req, rec)

    // Установка параметров пути
    c.SetPath("/users/:id")
    c.SetParamNames("id")
    c.SetParamValues("123")

    // Вызов handler
    err := getUser(c)

    // Проверки
    assert.NoError(t, err)
    assert.Equal(t, http.StatusOK, rec.Code)
}
```

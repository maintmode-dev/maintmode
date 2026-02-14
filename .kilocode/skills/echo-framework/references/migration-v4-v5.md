# Миграция с Echo v4 на v5

## Различия между v4 и v5

### Критические изменения

| Изменение | v4 | v5 |
|-----------|----|----|
| **Context** | `interface{}` | `struct` |
| **Handler signature** | `func(c echo.Context) error` | `func(c *echo.Context) error` |
| **Logger** | Кастомный интерфейс | `*slog.Logger` |
| **HTTPErrorHandler** | `func(err error, c Context)` | `func(c *Context, err error)` |
| **Route return** | `*Route` | `RouteInfo` |
| **Response()** | `*Response` | `http.ResponseWriter` |
| **HTTPError.Message** | `interface{}` | `string` |
| **Path params** | `ParamNames()/ParamValues()` | `PathValues()` |

### Новое в v5

1. **Generic helpers** — типобезопасное извлечение параметров
2. **PathValues** — структурированная работа с параметрами пути
3. **StartConfig** — унифицированная конфигурация сервера
4. **Router interface** — возможность кастомных роутеров
5. **echotest package** — утилиты для тестирования
6. **MiddlewareConfigurator** — интерфейс для конфигурации middleware

### Удалено в v5

1. `Logger()` middleware — заменён на `RequestLogger()`
2. `Timeout()` middleware — удалён
3. `CONNECT` константа — используйте `http.MethodConnect`
4. `MethodNotAllowedHandler` переменная
5. `NotFoundHandler` переменная
6. `StdLogger` поле в Echo
7. `Server`, `TLSServer` поля в Echo

## Рекомендации по миграции

### 1. Обновление зависимостей

```bash
# Замените импорты
find . -type f -name "*.go" -exec sed -i 's|echo/v4|echo/v5|g' {} +
```

### 2. Обновление сигнатур обработчиков

```bash
# Замените echo.Context на *echo.Context
find . -type f -name "*.go" -exec sed -i 's| echo.Context| *echo.Context|g' {} +
```

### 3. Обновление middleware

```go
// Было (v4)
e.Use(middleware.Logger())
e.Use(middleware.Timeout())

// Стало (v5)
e.Use(middleware.RequestLogger())
// Timeout удалён, используйте context.WithTimeout
```

### 4. Обновление обработчика ошибок

```go
// Было (v4)
e.HTTPErrorHandler = func(err error, c echo.Context) {
    // ...
}

// Стало (v5) - параметры переставлены!
e.HTTPErrorHandler = func(c *echo.Context, err error) {
    // ...
}

// Или используйте фабрику
e.HTTPErrorHandler = echo.DefaultHTTPErrorHandler(true)
```

### 5. Обновление работы с путями

```go
// Было (v4)
names := c.ParamNames()
values := c.ParamValues()

// Стало (v5)
pathValues := c.PathValues()
for _, pv := range pathValues {
    fmt.Println(pv.Name, pv.Value)
}
```

### 6. Обновление Binder

```go
// Было (v4)
func (b *MyBinder) Bind(i interface{}, c echo.Context) error {
    // ...
}

// Стало (v5) - параметры переставлены!
func (b *MyBinder) Bind(c *echo.Context, target any) error {
    // ...
}
```

### 7. Обновление Renderer

```go
// Было (v4)
func (t *TemplateRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
    // ...
}

// Стало (v5) - параметры переставлены!
func (t *TemplateRenderer) Render(c *echo.Context, w io.Writer, name string, data any) error {
    // ...
}
```

### 8. Обновление серверной конфигурации

```go
// Было (v4)
e.Start(":8080")
e.StartTLS(":443", "cert.pem", "key.pem")

// Стало (v5)
e.Start(":8080")

// Для расширенной конфигурации
ctx := context.Background()
sc := echo.StartConfig{Address: ":8080"}
sc.Start(ctx, e)
```

### 9. Обновление HTTPError

```go
// Было (v4)
return echo.NewHTTPError(400, "invalid", detail)

// Стало (v5) - только string
return echo.NewHTTPError(400, "invalid")
```

### 10. Обновление Response

```go
// Было (v4)
resp := c.Response()
resp.Header().Set("X-Custom", "value")

// Стало (v5) - возвращает http.ResponseWriter
c.Response().Header().Set("X-Custom", "value")

// Для доступа к *echo.Response
resp, err := echo.UnwrapResponse(c.Response())
```

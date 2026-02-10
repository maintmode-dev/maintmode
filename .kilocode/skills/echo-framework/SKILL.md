# Echo Framework Skill

## Описание

Echo — это высокопроизводительный, расширяемый и минималистичный веб-фреймворк для Go. Он предназначен для создания масштабируемых и высокопроизводительных веб-приложений и RESTful API.

### Ключевые особенности

- **Высокая производительность**: Оптимизированный HTTP-роутер на основе radix tree с нулевым динамическим распределением памяти
- **Минималистичный дизайн**: Простой и интуитивный API
- **Гибкая система middleware**: Поддержка middleware на уровне приложения, группы или отдельного маршрута
- **Централизованная обработка ошибок**: Единый механизм обработки HTTP-ошибок
- **Привязка данных**: Поддержка JSON, XML и form данных
- **Шаблонизация**: Поддержка любого движка шаблонов
- **HTTP/2 поддержка**: Встроенная поддержка HTTP/2
- **Автоматический TLS**: Интеграция с Let's Encrypt

---

## ⚠️ ВАЖНО: Несовместимость версий v4 и v5

**Echo v4 и v5 НЕСОВМЕСТИМЫ друг с другом.** Это критически важное изменение, которое требует внимания при миграции.

### Основные несовместимости

1. **Context изменился с интерфейса на структуру**
2. **Logger заменён на стандартный log/slog**
3. **Изменились сигнатуры большинства функций**
4. **Параметры в некоторых функциях переставлены местами**

---

## Версии Echo

### Поддерживаемые версии

- **Echo v5** — последняя мажорная версия (актуальна на 2026-01-18)
  - До 2026-03-31 критические проблемы с нарушением семантического версионирования будут исправляться
  - Для продакшена рекомендуется подождать до 2026-03-31 перед обновлением
- **Echo v4** — поддерживается с обновлениями безопасности и исправлениями багов до 2026-12-31

---

## Установка

### Echo v5

```bash
go get github.com/labstack/echo/v5
```

### Echo v4

```bash
go get github.com/labstack/echo/v4
```

---

## Основные концепции и компоненты

### 1. Echo Instance

Основной экземпляр приложения.

#### Echo v5

```go
package main

import (
    "github.com/labstack/echo/v5"
    "github.com/labstack/echo/v5/middleware"
    "log/slog"
    "net/http"
)

func main() {
    // Создание экземпляра Echo
    e := echo.New()

    // Middleware
    e.Use(middleware.RequestLogger())
    e.Use(middleware.Recover())

    // Роуты
    e.GET("/", hello)

    // Запуск сервера
    if err := e.Start(":8080"); err != nil {
        slog.Error("failed to start server", "error", err)
    }
}

// Handler - ВАЖНО: использует *echo.Context
func hello(c *echo.Context) error {
    return c.String(http.StatusOK, "Hello, World!")
}
```

#### Echo v4

```go
package main

import (
    "github.com/labstack/echo/v4"
    "github.com/labstack/echo/v4/middleware"
    "net/http"
)

func main() {
    // Создание экземпляра Echo
    e := echo.New()

    // Middleware
    e.Use(middleware.Logger())
    e.Use(middleware.Recover())

    // Роуты
    e.GET("/", hello)

    // Запуск сервера
    e.Logger.Fatal(e.Start(":8080"))
}

// Handler - использует echo.Context (без указателя)
func hello(c echo.Context) error {
    return c.String(http.StatusOK, "Hello, World!")
}
```

---

## 2. Routing (Маршрутизация)

Echo использует роутер на основе radix tree для быстрого поиска маршрутов.

### Базовые маршруты

#### Echo v5

```go
// Handler использует *echo.Context
e.GET("/users", getUsers)
e.POST("/users", createUser)
e.PUT("/users/:id", updateUser)
e.DELETE("/users/:id", deleteUser)

// Handler для всех методов
e.Any("/any", anyHandler)

// Handler для указанных методов
e.Match([]string{"GET", "POST"}, "/match", matchHandler)

func getUsers(c *echo.Context) error {
    return c.JSON(http.StatusOK, map[string]string{"message": "get users"})
}
```

#### Echo v4

```go
// Handler использует echo.Context
e.GET("/users", getUsers)
e.POST("/users", createUser)
e.PUT("/users/:id", updateUser)
e.DELETE("/users/:id", deleteUser)

func getUsers(c echo.Context) error {
    return c.JSON(http.StatusOK, map[string]string{"message": "get users"})
}
```

### Параметры пути

```go
// Параметр пути :id
e.GET("/users/:id", getUser)

// Wildcard - соответствует нулю или более символов
e.GET("/files/*", getAllFiles)

// В v5
func getUser(c *echo.Context) error {
    // Получение параметра
    id := c.Param("id")
    return c.String(http.StatusOK, "User ID: "+id)
}

// В v4
func getUser(c echo.Context) error {
    id := c.Param("id")
    return c.String(http.StatusOK, "User ID: "+id)
}
```

### Порядок совпадения маршрутов

1. Статические маршруты
2. Параметры маршрутов
3. Wildcard (match-any)

```go
e.GET("/users/new", newUser)      // 1. Сначала статический
e.GET("/users/:id", getUser)      // 2. Затем параметры
e.GET("/users/:id/files/*", getFiles) // 3. И wildcard
```

### Группы маршрутов

```go
// Создание группы с middleware
g := e.Group("/admin", middleware.BasicAuth(func(username, password string, c *echo.Context) (bool, error) {
    return username == "joe" && password == "secret", nil
}))

// Добавление middleware в группу
g.Use(middleware.CORS())

// Маршруты в группе
g.GET("/dashboard", dashboard)
g.POST("/settings", updateSettings)

// Вложенные группы
api := e.Group("/api")
v1 := api.Group("/v1")
v1.GET("/users", getUsers)
```

### Именование маршрутов

```go
// v5 - возвращает RouteInfo
route := e.GET("/users/:id", getUser)
route.Name = "get-user"

// или
e.POST("/users", createUser).Name = "create-user"

// v4 - возвращает *Route
route := e.GET("/users/:id", getUser)
route.Name = "get-user"
```

### Генерация URI

```go
// v5
func getUser(c *echo.Context) error {
    // Генерация URI по имени маршрута
    uri, err := e.Router().Routes().Reverse("get-user", 123)
    if err != nil {
        return err
    }
    return c.String(http.StatusOK, uri)
}

// v4
func getUser(c echo.Context) error {
    // Генерация URI по имени маршрута
    uri := e.Reverse("get-user", 123)
    return c.String(http.StatusOK, uri)
}
```

---

## 3. Context

Context представляет контекст текущего HTTP-запроса.

### ⚠️ КРИТИЧЕСКОЕ ИЗМЕНЕНИЕ: Context

#### Echo v5

```go
// Context - это КОНКРЕТНАЯ СТРУКТУРА
type Context struct {
    // Неэкспортируемые поля
}

// Handler использует УКАЗАТЕЛЬ на Context
func handler(c *echo.Context) error {
    // Доступ к запросу
    req := c.Request()

    // Доступ к ответу
    resp := c.Response()

    // Параметры пути
    id := c.Param("id")

    // Query параметры
    name := c.QueryParam("name")

    // Path values (новое в v5)
    pathValues := c.PathValues()
    for _, pv := range pathValues {
        fmt.Println(pv.Name, pv.Value)
    }

    return c.String(http.StatusOK, "OK")
}
```

#### Echo v4

```go
// Context - это ИНТЕРФЕЙС
type Context interface {
    Request() *http.Request
    Response() *Response
    // ... другие методы
}

// Handler использует Context (без указателя)
func handler(c echo.Context) error {
    req := c.Request()
    resp := c.Response()
    id := c.Param("id")
    name := c.QueryParam("name")
    return c.String(http.StatusOK, "OK")
}
```

### Методы Context

#### Общие для обеих версий

```go
// Запрос
c.Request() *http.Request

// Ответ
c.Response() http.ResponseWriter // v5 возвращает http.ResponseWriter
c.Response() *Response          // v4 возвращает *Response

// Параметры
c.Param(name string) string
c.QueryParam(name string) string
c.QueryParams() map[string][]string
c.FormValue(name string) string
c.FormFile(name string) (*multipart.FileHeader, error)

// Заголовки
c.Request().Header.Get("Content-Type")
c.Response().Header().Set("X-Custom", "value")

// Cookies
c.Cookie(name string) (*http.Cookie, error)
c.SetCookie(cookie *http.Cookie)

// Данные (context store)
c.Set(key string, val interface{})
c.Get(key string) interface{}

// Статус и размер ответа
c.Response().Status
c.Response().Size

// Отправка ответов
c.String(code int, s string) error
c.JSON(code int, i interface{}) error
c.JSONPretty(code int, i interface{}, indent string) error
c.JSONBlob(code int, b []byte) error
c.JSONP(code int, callback string, i interface{}) error
c.JSONPBlob(code int, callback string, b []byte) error
c.XML(code int, i interface{}) error
c.XMLPretty(code int, i interface{}, indent string) error
c.XMLBlob(code int, b []byte) error
c.Blob(code int, contentType string, b []byte) error
c.Stream(code int, contentType string, r io.Reader) error
c.File(file string) error
c.Attachment(file string, name string) error
c.Inline(file string, name string) error
c.NoContent(code int) error
c.Redirect(code int, url string) error
```

#### Специфичные для v5

```go
// Path values (заменяет ParamNames/ParamValues)
c.PathValues() PathValues
c.SetPathValues(pathValues PathValues)

// Методы с дефолтными значениями
c.ParamOr(name, defaultValue string) string
c.QueryParamOr(name, defaultValue string) string
c.FormValueOr(name, defaultValue string) string

// Информация о маршруте
c.RouteInfo() RouteInfo

// Файлы с filesystem
c.FileFS(file string, filesystem fs.FS) error

// Logger возвращает *slog.Logger
c.Logger() *slog.Logger
c.SetLogger(logger *slog.Logger)
```

#### Специфичные для v4

```go
// Param names и values
c.ParamNames() []string
c.ParamValues() []string
c.SetParamNames(names ...string)
c.SetParamValues(values ...string)

// Logger возвращает Logger (интерфейс)
c.Logger() Logger
```

---

## 4. Middleware

Middleware — это функции, которые выполняются до или после обработчика запроса.

### Создание кастомного middleware

#### Echo v5

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

#### Echo v4

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

### Встроенные middleware

#### Logger

```go
// v5
e.Use(middleware.RequestLogger())

// v4
e.Use(middleware.Logger())
```

#### Recover

```go
// v5 и v4 (одинаково)
e.Use(middleware.Recover())
```

#### CORS

```go
// v5 - принимает origins
e.Use(middleware.CORS("https://example.com", "https://api.example.com"))

// v4
e.Use(middleware.CORS())
```

#### Body Limit

```go
// v5 - принимает байты
e.Use(middleware.BodyLimit(10 * 1024 * 1024)) // 10MB

// v4
e.Use(middleware.BodyLimit("10M"))
```

#### Gzip

```go
// v5 и v4 (одинаково)
e.Use(middleware.Gzip())
```

#### Basic Auth

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

#### JWT

```go
// v5
e.Use(middleware.JWT([]byte("secret")))

// v4
e.Use(middleware.JWT([]byte("secret")))
```

#### Rate Limiter

```go
// v5
e.Use(middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(100)))

// v4
e.Use(middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(100)))
```

---

## 5. Binding (Привязка данных)

Echo предоставляет несколько способов привязки данных из HTTP-запроса.

### Struct Tag Binding

#### Echo v5

```go
type UserDTO struct {
    Name  string `json:"name" form:"name" query:"name"`
    Email string `json:"email" form:"email" query:"email"`
    Age   int    `json:"age" form:"age" query:"age"`
}

func createUser(c *echo.Context) error {
    u := new(UserDTO)
    if err := c.Bind(u); err != nil {
        return c.String(http.StatusBadRequest, "bad request")
    }

    // Безопасность: маппинг на бизнес-структуру
    user := User{
        Name:  u.Name,
        Email: u.Email,
        Age:   u.Age,
    }

    return c.JSON(http.StatusOK, user)
}
```

#### Echo v4

```go
type UserDTO struct {
    Name  string `json:"name" form:"name" query:"name"`
    Email string `json:"email" form:"email" query:"email"`
    Age   int    `json:"age" form:"age" query:"age"`
}

func createUser(c echo.Context) error {
    u := new(UserDTO)
    if err := c.Bind(u); err != nil {
        return c.String(http.StatusBadRequest, "bad request")
    }

    return c.JSON(http.StatusOK, u)
}
```

### Теги источников данных

- `query` — query параметры
- `param` — параметры пути
- `header` — заголовки
- `json` — тело запроса (JSON)
- `xml` — тело запроса (XML)
- `form` — form данные (из query и body)

### Direct Source Binding

#### Echo v5

```go
// Тело запроса
err := echo.BindBody(c, &payload)

// Query параметры
err := echo.BindQueryParams(c, &payload)

// Параметры пути
err := echo.BindPathValues(c, &payload)

// Заголовки
err := echo.BindHeaders(c, &payload)
```

#### Echo v4

```go
// Тело запроса
err := echo.BindBody(c, &payload)

// Query параметры
err := echo.BindQueryParams(c, &payload)

// Параметры пути (называется BindPathParams)
err := echo.BindPathParams(c, &payload)
```

### Fluent Binding

#### Echo v5

```go
// URL: /api/search?active=true&id=1&id=2&id=3&length=25
var opts struct {
    IDs    []int64
    Active bool
}

length := int64(50) // дефолтное значение

// QueryParamsBinder использует *echo.Context
err := echo.QueryParamsBinder(c).
    Int64("length", &length).
    Int64s("ids", &opts.IDs).
    Bool("active", &opts.Active).
    BindError()
```

#### Echo v4

```go
// QueryParamsBinder использует echo.Context
err := echo.QueryParamsBinder(c).
    Int64("length", &length).
    Int64s("ids", &opts.IDs).
    Bool("active", &opts.Active).
    BindError()
```

### Generic Parameter Extraction (v5)

Echo v5 предоставляет типобезопасные функции для извлечения параметров:

```go
// Query параметры
id, err := echo.QueryParam[int](c, "id")
page, err := echo.QueryParamOr[int](c, "page", 1)
tags, err := echo.QueryParams[string](c, "tags")

// Параметры пути
userId, err := echo.PathParam[int](c, "id")

// Form значения
username, err := echo.FormValue[string](c, "username")
emails, err := echo.FormValues[string](c, "emails")

// Context store
user, err := echo.ContextGet[*User](c, "user")
count, err := echo.ContextGetOr[int](c, "count", 0)
```

---

## 6. Error Handling (Обработка ошибок)

Echo поддерживает централизованную обработку ошибок.

### Возврат ошибок

```go
// Создание HTTP ошибки
return echo.NewHTTPError(http.StatusBadRequest, "invalid input")

// Без сообщения (используется статусный текст)
return echo.NewHTTPError(http.StatusUnauthorized)
```

### Кастомный обработчик ошибок

#### Echo v5

```go
// ⚠️ Параметры переставлены местами!
// v5: (c *echo.Context, err error)
func customHTTPErrorHandler(c *echo.Context, err error) {
    if c.Response().Committed {
        return
    }

    code := http.StatusInternalServerError
    if he, ok := err.(*echo.HTTPError); ok {
        code = he.Code
    }

    c.Logger().Error(err)

    errorPage := fmt.Sprintf("%d.html", code)
    if err := c.File(errorPage); err != nil {
        c.Logger().Error(err)
    }
}

// Или использовать фабрику
e.HTTPErrorHandler = echo.DefaultHTTPErrorHandler(true) // exposeError=true
```

#### Echo v4

```go
// v4: (err error, c echo.Context)
func customHTTPErrorHandler(err error, c echo.Context) {
    if c.Response().Committed {
        return
    }

    code := http.StatusInternalServerError
    if he, ok := err.(*echo.HTTPError); ok {
        code = he.Code
    }

    c.Logger().Error(err)

    errorPage := fmt.Sprintf("%d.html", code)
    if err := c.File(errorPage); err != nil {
        c.Logger().Error(err)
    }
}

e.HTTPErrorHandler = customHTTPErrorHandler
```

---

## 7. Server Configuration (Конфигурация сервера)

### Echo v5

```go
// Простой запуск
e.Start(":8080")

// Расширенная конфигурация с StartConfig
ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer cancel()

sc := echo.StartConfig{
    Address:          ":8080",
    HideBanner:       true,
    HidePort:         false,
    GracefulTimeout: 10 * time.Second,
    TLSConfig:        &tls.Config{},
}

if err := sc.Start(ctx, e); err != nil {
    log.Fatal(err)
}

// TLS
sc.StartTLS(ctx, e, "cert.pem", "key.pem")
```

### Echo v4

```go
// Простой запуск
e.Start(":8080")

// TLS
e.StartTLS(":443", "cert.pem", "key.pem")

// Auto TLS
e.StartAutoTLS(":443")

// Graceful shutdown
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
if err := e.Shutdown(ctx); err != nil {
    e.Logger.Fatal(err)
}
```

---

## 8. Logger

### Echo v5

```go
import "log/slog"

// Echo использует стандартный log/slog
e.Logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))

// В handler
func handler(c *echo.Context) error {
    c.Logger().Info("processing request", "path", c.Request().URL.Path)
    c.Logger().Error("something went wrong", "error", err)
    return nil
}
```

### Echo v4

```go
// Echo использует кастомный Logger интерфейс
e.Logger.SetLevel(log.INFO)
e.Logger.SetOutput(os.Stdout)

// В handler
func handler(c echo.Context) error {
    c.Logger().Info("processing request")
    c.Logger().Error("something went wrong")
    return nil
}
```

---

## 9. Static Files

### Echo v5

```go
// Возвращает RouteInfo
e.Static("/static", "assets")
e.StaticFS("/static", os.DirFS("assets"))

// С middleware
e.Static("/static", "assets", middleware.Gzip())

// Отдельный файл
e.File("/favicon.ico", "favicon.ico")
e.FileFS("/robots.txt", "robots.txt", os.DirFS("."))
```

### Echo v4

```go
// Возвращает *Route
e.Static("/static", "assets")
e.StaticFS("/static", os.DirFS("assets"))

// Отдельный файл
e.File("/favicon.ico", "favicon.ico")
e.FileFS("/robots.txt", "robots.txt", os.DirFS("."))
```

---

## 10. Templates

### Echo v5

```go
import "html/template"

// Renderer интерфейс
type TemplateRenderer struct {
    templates *template.Template
}

func (t *TemplateRenderer) Render(c *echo.Context, w io.Writer, name string, data any) error {
    return t.templates.ExecuteTemplate(w, name, data)
}

// Использование
renderer := &TemplateRenderer{
    templates: template.Must(template.ParseGlob("views/*.html")),
}
e.Renderer = renderer

// В handler
func handler(c *echo.Context) error {
    return c.Render(http.StatusOK, "index.html", map[string]interface{}{
        "title": "Home",
    })
}
```

### Echo v4

```go
import "html/template"

// Renderer интерфейс (параметры в другом порядке)
type TemplateRenderer struct {
    templates *template.Template
}

func (t *TemplateRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
    return t.templates.ExecuteTemplate(w, name, data)
}

// Использование
renderer := &TemplateRenderer{
    templates: template.Must(template.ParseGlob("views/*.html")),
}
e.Renderer = renderer

// В handler
func handler(c echo.Context) error {
    return c.Render(http.StatusOK, "index.html", map[string]interface{}{
        "title": "Home",
    })
}
```

---

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

---

## Рекомендации по миграции с v4 на v5

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

---

## Лучшие практики

### 1. Используйте группы маршрутов

```go
// API v1
v1 := e.Group("/api/v1")
v1.GET("/users", getUsers)
v1.POST("/users", createUser)

// Admin routes
admin := e.Group("/admin", adminAuthMiddleware)
admin.GET("/dashboard", dashboard)
```

### 2. Централизованная обработка ошибок

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

### 3. Валидация входных данных

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

### 4. Используйте middleware для общих задач

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

### 5. Безопасность при binding

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

### 6. Используйте context для передачи данных между middleware

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

### 7. Graceful shutdown

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

### 8. Структура проекта

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

### 9. Конфигурация

```go
// v5 - используйте Config
config := echo.Config{
    Logger:   slog.New(slog.NewJSONHandler(os.Stdout, nil)),
    Binder:   &CustomBinder{},
    Renderer: &TemplateRenderer{},
}

e := echo.NewWithConfig(config)
```

### 10. Тестирование

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

---

## Официальные ресурсы

- **Официальный сайт**: https://echo.labstack.com
- **Документация**: https://echo.labstack.com/docs
- **GitHub репозиторий**: https://github.com/labstack/echo
- **GitHub Discussions**: https://github.com/labstack/echo/discussions

## Официальные middleware репозитории

| Репозиторий | Описание |
|-------------|----------|
| github.com/labstack/echo-jwt | JWT middleware |
| github.com/labstack/echo-contrib | casbin, gorilla/sessions, jaegertracing, prometheus, pprof, zipkin |

## Сторонние библиотеки

| Репозиторий | Описание |
|-------------|----------|
| deepmap/oapi-codegen | OpenAPI генератор кода |
| github.com/swaggo/echo-swagger | Swagger 2.0 документация |
| github.com/ziflex/lecho | Zerolog logger wrapper |
| github.com/brpaz/echozap | Zap logger wrapper |
| github.com/samber/slog-echo | slog logger wrapper |

---

## Требования к Go версии

- **Echo v4**: Go 1.24.0+
- **Echo v5**: Go 1.25.0+

---

## Лицензия

MIT License

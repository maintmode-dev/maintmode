# Context

Context представляет контекст текущего HTTP-запроса.

## ⚠️ КРИТИЧЕСКОЕ ИЗМЕНЕНИЕ: Context

### Echo v5

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

### Echo v4

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

## Методы Context

### Общие для обеих версий

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

### Специфичные для v5

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

### Специфичные для v4

```go
// Param names и values
c.ParamNames() []string
c.ParamValues() []string
c.SetParamNames(names ...string)
c.SetParamValues(values ...string)

// Logger возвращает Logger (интерфейс)
c.Logger() Logger
```

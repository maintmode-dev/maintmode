# Error Handling (Обработка ошибок)

Echo поддерживает централизованную обработку ошибок.

## Возврат ошибок

```go
// Создание HTTP ошибки
return echo.NewHTTPError(http.StatusBadRequest, "invalid input")

// Без сообщения (используется статусный текст)
return echo.NewHTTPError(http.StatusUnauthorized)
```

## Кастомный обработчик ошибок

### Echo v5

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

### Echo v4

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

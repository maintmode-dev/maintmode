# Binding (Привязка данных)

Echo предоставляет несколько способов привязки данных из HTTP-запроса.

## Struct Tag Binding

### Echo v5

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

### Echo v4

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

## Теги источников данных

- `query` — query параметры
- `param` — параметры пути
- `header` — заголовки
- `json` — тело запроса (JSON)
- `xml` — тело запроса (XML)
- `form` — form данные (из query и body)

## Direct Source Binding

### Echo v5

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

### Echo v4

```go
// Тело запроса
err := echo.BindBody(c, &payload)

// Query параметры
err := echo.BindQueryParams(c, &payload)

// Параметры пути (называется BindPathParams)
err := echo.BindPathParams(c, &payload)
```

## Fluent Binding

### Echo v5

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

### Echo v4

```go
// QueryParamsBinder использует echo.Context
err := echo.QueryParamsBinder(c).
    Int64("length", &length).
    Int64s("ids", &opts.IDs).
    Bool("active", &opts.Active).
    BindError()
```

## Generic Parameter Extraction (v5)

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

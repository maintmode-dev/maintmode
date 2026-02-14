# Routing (Маршрутизация)

Echo использует роутер на основе radix tree для быстрого поиска маршрутов.

## Базовые маршруты

### Echo v5

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

### Echo v4

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

## Параметры пути

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

## Порядок совпадения маршрутов

1. Статические маршруты
2. Параметры маршрутов
3. Wildcard (match-any)

```go
e.GET("/users/new", newUser)      // 1. Сначала статический
e.GET("/users/:id", getUser)      // 2. Затем параметры
e.GET("/users/:id/files/*", getFiles) // 3. И wildcard
```

## Группы маршрутов

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

## Именование маршрутов

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

## Генерация URI

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

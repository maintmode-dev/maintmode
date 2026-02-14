# Logger

## Echo v5

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

## Echo v4

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

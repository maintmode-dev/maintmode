# Static Files и Templates

## Static Files

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

## Templates

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

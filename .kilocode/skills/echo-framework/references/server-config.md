# Server Configuration (Конфигурация сервера)

## Echo v5

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

## Echo v4

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

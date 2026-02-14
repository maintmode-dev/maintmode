# Rotation логов

## Использование lumberjack

### Установка

```bash
go get gopkg.in/natefinch/lumberjack.v2
```

### Базовая настройка rotation

```go
import (
    "gopkg.in/natefinch/lumberjack.v2"
    "go.uber.org/zap"
    "go.uber.org/zap/zapcore"
)

func NewLoggerWithRotation(logPath string) *zap.Logger {
    writer := &lumberjack.Logger{
        Filename:   logPath,
        MaxSize:    100, // megabytes
        MaxBackups: 3,
        MaxAge:     28, // days
        Compress:   true,
    }

    core := zapcore.NewCore(
        zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
        zapcore.AddSync(writer),
        zapcore.InfoLevel,
    )

    return zap.New(core, zap.AddCaller())
}
```

## Параметры lumberjack

### MaxSize

Максимальный размер файла в мегабайтах перед rotation:

```go
writer := &lumberjack.Logger{
    Filename: "logs/app.log",
    MaxSize:  100, // 100 MB
}
```

**Рекомендации:**
- Dev: 10-50 MB
- Production: 100-500 MB

### MaxBackups

Количество старых файлов для хранения:

```go
writer := &lumberjack.Logger{
    Filename:   "logs/app.log",
    MaxBackups: 3, // Храним 3 старых файла
}
```

**Пример результата:**
```
app.log          # текущий
app.log.1        # предыдущий
app.log.2        # еще более старый
app.log.3        # самый старый
```

### MaxAge

Максимальный возраст файла в днях:

```go
writer := &lumberjack.Logger{
    Filename: "logs/app.log",
    MaxAge:   28, // 28 дней
}
```

### Compress

Сжатие старых файлов:

```go
writer := &lumberjack.Logger{
    Filename: "logs/app.log",
    Compress: true, // Сжимать в .gz
}
```

**Результат:**
```
app.log
app.log.1.gz
app.log.2.gz
```

## Конфигурация для dev и prod

### Development

```go
func NewDevLogger() *zap.Logger {
    config := zap.NewDevelopmentConfig()
    config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
    config.OutputPaths = []string{"stdout"}

    logger, _ := config.Build()
    return logger
}
```

### Production с rotation

```go
func NewProdLogger() *zap.Logger {
    // Rotation для основных логов
    appWriter := &lumberjack.Logger{
        Filename:   "logs/app.log",
        MaxSize:    100,
        MaxBackups: 3,
        MaxAge:     28,
        Compress:   true,
    }

    // Rotation для ошибок
    errorWriter := &lumberjack.Logger{
        Filename:   "logs/error.log",
        MaxSize:    100,
        MaxBackups: 3,
        MaxAge:     28,
        Compress:   true,
    }

    // Encoder config
    encoderConfig := zap.NewProductionEncoderConfig()
    encoderConfig.TimeKey = "time"
    encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

    // Core для info и выше
    infoCore := zapcore.NewCore(
        zapcore.NewJSONEncoder(encoderConfig),
        zapcore.AddSync(appWriter),
        zapcore.InfoLevel,
    )

    // Core только для ошибок
    errorCore := zapcore.NewCore(
        zapcore.NewJSONEncoder(encoderConfig),
        zapcore.AddSync(errorWriter),
        zapcore.ErrorLevel,
    )

    // Комбинированный core
    core := zapcore.NewTee(infoCore, errorCore)

    return zap.New(core,
        zap.AddCaller(),
        zap.AddStacktrace(zapcore.ErrorLevel),
    )
}
```

## Управление размером лог-файлов

### Автоматическое управление

Lumberjack автоматически:
- Создает новый файл при достижении MaxSize
- Удаляет старые файлы по MaxBackups или MaxAge
- Сжимает файлы если Compress = true

### Ручное управление

```go
writer := &lumberjack.Logger{
    Filename: "logs/app.log",
}

// Принудительная rotation
writer.Rotate()
```

## Мониторинг размера логов

```bash
# Размер текущих логов
du -h logs/

# Размер с учетом сжатия
du -h logs/*.gz

# Количество файлов
ls -1 logs/ | wc -l

# Размер за последние 7 дней
find logs/ -mtime -7 -exec du -ch {} + | grep total
```

## Полный пример с Environment

```go
package config

import (
    "gopkg.in/natefinch/lumberjack.v2"
    "go.uber.org/zap"
    "go.uber.org/zap/zapcore"
)

func NewLogger(env Environment) *zap.Logger {
    if env.IsDev() {
        return newDevLogger()
    }
    return newProdLogger()
}

func newDevLogger() *zap.Logger {
    config := zap.NewDevelopmentConfig()
    config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
    logger, _ := config.Build()
    return logger
}

func newProdLogger() *zap.Logger {
    appWriter := &lumberjack.Logger{
        Filename:   "logs/app.log",
        MaxSize:    100,
        MaxBackups: 3,
        MaxAge:     28,
        Compress:   true,
    }

    errorWriter := &lumberjack.Logger{
        Filename:   "logs/error.log",
        MaxSize:    100,
        MaxBackups: 3,
        MaxAge:     28,
        Compress:   true,
    }

    encoderConfig := zap.NewProductionEncoderConfig()
    encoderConfig.TimeKey = "time"
    encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

    infoCore := zapcore.NewCore(
        zapcore.NewJSONEncoder(encoderConfig),
        zapcore.AddSync(appWriter),
        zapcore.InfoLevel,
    )

    errorCore := zapcore.NewCore(
        zapcore.NewJSONEncoder(encoderConfig),
        zapcore.AddSync(errorWriter),
        zapcore.ErrorLevel,
    )

    core := zapcore.NewTee(infoCore, errorCore)

    return zap.New(core,
        zap.AddCaller(),
        zap.AddStacktrace(zapcore.ErrorLevel),
    )
}
```

## Best Practices

1. **Разделяйте логи** - app.log для всего, error.log для ошибок
2. **Используйте compression** - экономит место на диске
3. **Настройте MaxAge** - автоматическое удаление старых логов
4. **Мониторьте размер** - следите за disk usage
5. **Dev без rotation** - в development логи в stdout удобнее

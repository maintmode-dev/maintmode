---
name: docker-go-apps
description: Docker для Go приложений с multi-stage builds, оптимизацией и .dockerignore. Используй этот скилл, когда нужно создавать Dockerfile для Go приложений, оптимизировать образы, настраивать docker-compose для Go сервисов.
license: MIT
metadata:
  category: development
  source:
    repository: project-specific
    path: docker-go-apps
---

# Docker для Go приложений

## Описание
Этот скилл предоставляет руководство по созданию оптимизированных Docker-образов для Go приложений, включая multi-stage builds, использование .dockerignore, интеграцию с docker-compose и лучшие практики по оптимизации размера и производительности образов.

## Когда использовать
Используй этот скилл, когда нужно:
- Создавать Dockerfile для Go приложения
- Настраивать multi-stage builds для оптимизации размера образа
- Создавать .dockerignore для исключения ненужных файлов
- Настраивать docker-compose для Go сервисов
- Оптимизировать сборку и кэширование слоёв
- Интегрировать Go приложение с PostgreSQL, Redis и другими сервисами

## Multi-stage Builds

### Базовый multi-stage Dockerfile

Multi-stage builds позволяют разделить процесс сборки на несколько этапов, что значительно уменьшает размер финального образа.

```dockerfile
# --- Stage 0: base ---
FROM golang:1.25-alpine AS base
ARG GOBIN=/app/bin
WORKDIR /app
RUN apk add --no-cache make git

COPY Makefile ./
RUN ENVIRONMENT=production > .env.local
RUN make bin-deps-build

# --- Stage 1: build backend (Go) ---
FROM base AS builder
ARG GOBIN=/app/bin
WORKDIR /app

COPY .goreleaser.yaml ./
COPY Makefile ./
COPY docs.go ./
COPY web/ web/
COPY cmd/ cmd/
COPY internal/ internal/
COPY docs/ docs/
COPY test/ test/
COPY go.mod go.sum ./

RUN make deps
RUN make build args="--id main --output=/app/bin/maintmode"

# --- Stage 2: production image ---
FROM alpine:latest
WORKDIR /app/

RUN apk --no-cache add ca-certificates

COPY --from=builder /app/bin/maintmode .

EXPOSE 8080

CMD ["./maintmode"]
```

### Оптимизация кэширования слоёв

Для ускорения сборки копируйте файлы в порядке от наименее изменяемых к наиболее изменяемым:

```dockerfile
# Сначала копируем go.mod и go.sum - они меняются редко
COPY go.mod go.sum ./
RUN go mod download

# Затем копируем исходный код
COPY . .

# Сборка
RUN CGO_ENABLED=0 go build -o /app/main ./cmd/app
```

### Использование .buildargs для конфигурации

```dockerfile
ARG GOBIN=/app/bin
ARG ENVIRONMENT=production

ENV ENVIRONMENT=${ENVIRONMENT}
```

## .dockerignore

### Базовый .dockerignore

Файл `.dockerignore` исключает ненужные файлы из контекста сборки, ускоряя процесс и уменьшая размер образа:

```
# Git
.git
.gitignore

# Documentation
README.md
docs/
*.md

# IDE
.vscode/
.idea/
*.swp
*.swo
*~

# Build artifacts
bin/
dist/
*.exe
*.dll
*.so
*.dylib

# Test files
_test.go
test/
testdata/

# CI/CD
.github/
.gitlab-ci.yml
.travis.yml

# Docker
Dockerfile*
docker-compose*.yml
.dockerignore

# Environment
.env
.env.*
!.env.example

# Logs
*.log
logs/

# Temporary files
tmp/
temp/
*.tmp

# OS files
.DS_Store
Thumbs.db

# Air live reload
.air.toml
```

## Docker Compose

### Базовая конфигурация с PostgreSQL

```yaml
services:
  postgres:
    image: postgres:18-alpine
    container_name: postgres
    restart: unless-stopped
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: maintmode
    ports:
      - "5432:5432"
    volumes:
      - db_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 5s
      timeout: 5s
      retries: 5

volumes:
  db_data:

networks:
  theapp:
    driver: bridge
    name: theapp
```

### Конфигурация приложения с зависимостями

```yaml
services:
  pg_doorman:
    image: ghcr.io/ozontech/pg_doorman:latest
    container_name: pg_doorman
    restart: unless-stopped
    command: ["pg_doorman", "-l", "info", "/etc/pg_doorman/pg_doorman.toml"]
    ports:
      - "6432:6432"
      - "9127:9127"
    volumes:
      - ./config/pg_doorman.toml:/etc/pg_doorman/pg_doorman.toml:ro
    depends_on:
      postgres:
        condition: service_healthy

  maintmode:
    build:
      context: .
      dockerfile: ./.build/Dockerfile
    container_name: maintmode
    restart: unless-stopped
    ports:
      - "8000:8000"
      - "8001:8001"
    environment:
      ENVIRONMENT: dev
      DB_DSN: postgres://postgres:postgres@pg_doorman:6432/maintmode?sslmode=disable
      DB_DRIVER: postgres
      DB_MAX_OPEN_CONNS: 50
      DB_MAX_IDLE_CONNS: 20
      DB_CONNECTIONS_MAX_LIFETIME: 10m
      DB_CONNECTION_MAX_IDLE_TIME: 5m
    depends_on:
      postgres:
        condition: service_healthy
      pg_doorman:
        condition: service_started
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8001/readiness"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
```

### Health Checks

Health checks позволяют Docker отслеживать состояние контейнера:

```yaml
healthcheck:
  test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8001/readiness"]
  interval: 30s      # Интервал между проверками
  timeout: 10s       # Таймаут для каждой проверки
  retries: 3         # Количество неудачных попыток перед пометкой как unhealthy
  start_period: 40s  # Время до начала проверок после запуска
```

### Зависимости между сервисами

Используйте `depends_on` с `condition` для корректного порядка запуска:

```yaml
depends_on:
  postgres:
    condition: service_healthy  # Ждёт, пока healthcheck пройдёт
  pg_doorman:
    condition: service_started  # Ждёт только запуска контейнера
```

## Оптимизация образов

### Использование Alpine Linux

Alpine Linux значительно уменьшает размер образа:

```dockerfile
# Для сборки
FROM golang:1.25-alpine AS builder

# Для production
FROM alpine:latest
```

### Установка только необходимых зависимостей

```dockerfile
RUN apk --no-cache add ca-certificates tzdata
```

### Копирование только бинарного файла

```dockerfile
COPY --from=builder /app/bin/maintmode .
```

### Использование .dockerignore

Правильный `.dockerignore` исключает ненужные файлы из контекста сборки.

## Makefile интеграция

### Docker команды в Makefile

```makefile
# docker-up - Start all database services
.PHONY: docker-up
docker-up:
	docker-compose up -d
	make docker-ps

# docker-down - Stop and remove all database containers
.PHONY: docker-down
docker-down:
	docker-compose down -v --remove-orphans
	make docker-ps

# docker-logs - Stream logs from all database containers
.PHONY: docker-logs
docker-logs:
	docker-compose logs -f

# docker-ps - Show status of database containers
.PHONY: docker-ps
docker-ps:
	docker-compose ps -a

# app-up - Start all services with maintmode
.PHONY: app-up
app-up:
	docker-compose -f compose.yaml -f compose.app.yaml up -d
	make app-ps

# app-down - Stop and remove all containers
.PHONY: app-down
app-down:
	docker-compose -f compose.yaml -f compose.app.yaml down -v --remove-orphans
	make app-ps

# app-logs - Stream logs from maintmode container
.PHONY: app-logs
app-logs:
	docker-compose -f compose.yaml -f compose.app.yaml logs -f maintmode

# app-ps - Show status of all containers
.PHONY: app-ps
app-ps:
	docker-compose -f compose.yaml -f compose.app.yaml ps -a
```

## Полезные команды

### Сборка образа

```bash
docker build -f .build/Dockerfile -t maintmode:latest .
```

### Сборка с аргументами

```bash
docker build --build-arg ENVIRONMENT=production -f .build/Dockerfile -t maintmode:prod .
```

### Запуск контейнера

```bash
docker run -p 8080:8080 maintmode:latest
```

### Просмотр логов

```bash
docker logs -f maintmode
```

### Вход в контейнер

```bash
docker exec -it maintmode sh
```

### Очистка неиспользуемых ресурсов

```bash
docker system prune -a
```

## Рекомендации

1. **Всегда используйте multi-stage builds** для production образов Go приложений
2. **Копируйте go.mod и go.sum отдельно** для эффективного кэширования слоёв
3. **Используйте Alpine Linux** для финального образа для минимизации размера
4. **Настраивайте health checks** для всех сервисов
5. **Используйте .dockerignore** для исключения ненужных файлов
6. **Определяйте зависимости** между сервисами в docker-compose
7. **Используйте volumes** для персистентных данных (базы данных)
8. **Настраивайте restart policy** для production контейнеров

## Ресурсы

- [Dockerfile Best Practices](https://docs.docker.com/develop/develop-images/dockerfile_best-practices/)
- [Multi-stage builds](https://docs.docker.com/build/building/multi-stage/)
- [Docker Compose Documentation](https://docs.docker.com/compose/)
- [Go Docker Images](https://hub.docker.com/_/golang)

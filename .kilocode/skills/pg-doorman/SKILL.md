---
name: pg-doorman
description: PostgreSQL connection pooler (конфигурация, pool_mode, мониторинг). Используй этот скилл, когда нужно настраивать pg_doorman для управления пулом подключений к PostgreSQL, настраивать режимы пулинга и мониторинг.
license: MIT
metadata:
  category: development
  source:
    repository: project-specific
    path: pg-doorman
---

# PostgreSQL Connection Pooler - pg_doorman

## Описание
Этот скилл предоставляет руководство по настройке и использованию pg_doorman - высокопроизводительного connection pooler для PostgreSQL от Ozon Tech. Включает конфигурацию, режимы пулинга, мониторинг через Prometheus и интеграцию с docker-compose.

## Когда использовать
Используй этот скилл, когда нужно:
- Настраивать pg_doorman для управления пулом подключений
- Выбирать и настраивать pool_mode (Session, Transaction, Statement)
- Настраивать мониторинг через Prometheus
- Интегрировать pg_doorman с docker-compose
- Оптимизировать производительность подключений к PostgreSQL
- Настраивать health checks и таймауты

## Что такое pg_doorman

pg_doorman - это connection pooler для PostgreSQL, который:
- Управляет пулом подключений к базе данных
- Уменьшает нагрузку на PostgreSQL
- Предоставляет метрики для мониторинга через Prometheus
- Поддерживает несколько режимов пулинга
- Разработан и используется в Ozon для высоконагруженных систем

## Установка и запуск

### Через Docker Compose

```yaml
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
```

### Команда запуска

```bash
docker run -d \
  --name pg_doorman \
  -p 6432:6432 \
  -p 9127:9127 \
  -v $(pwd)/config/pg_doorman.toml:/etc/pg_doorman/pg_doorman.toml:ro \
  ghcr.io/ozontech/pg_doorman:latest \
  pg_doorman -l info /etc/pg_doorman/pg_doorman.toml
```

## Конфигурация pg_doorman.toml

### Полная конфигурация

```toml
[general]
host = "0.0.0.0"
port = 6432
connect_timeout = 3000
query_wait_timeout = 5000
idle_timeout = 300000000
tcp_keepalives_idle = 5
tcp_keepalives_count = 5
tcp_keepalives_interval = 5
tcp_so_linger = 0
tcp_no_delay = true
unix_socket_buffer_size = 1048576
log_client_connections = true
log_client_disconnections = true
shutdown_timeout = 10000
message_size_to_be_steam = 1048576
max_memory_usage = 268435456
max_connections = 8192
max_concurrent_creates = 4
server_lifetime = 300000
retain_connections_time = 60000
server_round_robin = false
sync_server_parameters = false
worker_threads = 4
proxy_copy_data_timeout = 15000
worker_cpu_affinity_pinning = false
backlog = 0
pooler_check_query = ";"
tls_rate_limit_per_second = 0
server_tls = false
verify_server_certificate = false
admin_username = "admin"
admin_password = "admin"
prepared_statements = true
prepared_statements_cache_size = 8192
client_prepared_statements_cache_size = 0
daemon_pid_file = "/tmp/pg_doorman.pid"

[prometheus]
host = "0.0.0.0"
port = 9127
enabled = true

[pools.maintmode]
pool_mode = "Transaction"
cleanup_server_connections = false
log_client_parameter_status_changes = false
server_host = "postgres"
server_port = 5432
server_database = "maintmode"

[[pools.maintmode.users]]
username = "postgres"
password = "md53175bce1d3201d16594cebf9d7eb3f9d"
server_username = "postgres"
server_password = "postgres"
pool_size = 40
```

### Раздел [general]

Основные настройки pg_doorman:

| Параметр | Описание | Значение по умолчанию |
|----------|----------|----------------------|
| `host` | Хост для прослушивания | 0.0.0.0 |
| `port` | Порт для подключений клиентов | 6432 |
| `connect_timeout` | Таймаут подключения к серверу (мс) | 3000 |
| `query_wait_timeout` | Таймаут ожидания выполнения запроса (мс) | 5000 |
| `idle_timeout` | Таймаут неактивного подключения (нс) | 300000000 |
| `max_connections` | Максимальное количество клиентских подключений | 8192 |
| `max_memory_usage` | Максимальное использование памяти (байт) | 268435456 |
| `worker_threads` | Количество рабочих потоков | 4 |
| `prepared_statements` | Использовать prepared statements | true |

### Раздел [prometheus]

Настройки Prometheus мониторинга:

| Параметр | Описание | Значение по умолчанию |
|----------|----------|----------------------|
| `host` | Хост для метрик | 0.0.0.0 |
| `port` | Порт для метрик | 9127 |
| `enabled` | Включить метрики | true |

### Раздел [pools]

Настройки пулов подключений:

| Параметр | Описание | Значение по умолчанию |
|----------|----------|----------------------|
| `pool_mode` | Режим пулинга (Session/Transaction/Statement) | Transaction |
| `server_host` | Хост PostgreSQL | postgres |
| `server_port` | Порт PostgreSQL | 5432 |
| `server_database` | Имя базы данных | maintmode |
| `cleanup_server_connections` | Очищать серверные подключения | false |

## Режимы пулинга (pool_mode)

### Session Mode

Каждое клиентское подключение получает одно серверное подключение на всю сессию:

```toml
[pools.maintmode]
pool_mode = "Session"
```

**Когда использовать:**
- Приложения, требующие постоянного соединения
- Использование prepared statements на уровне сессии
- Транзакции с длительным временем выполнения

**Плюсы:**
- Прямое соответствие клиентского и серверного подключений
- Полная совместимость с PostgreSQL

**Минусы:**
- Меньшая эффективность использования подключений
- Меньше возможностей для масштабирования

### Transaction Mode (рекомендуется)

Серверное подключение назначается клиенту только на время транзакции:

```toml
[pools.maintmode]
pool_mode = "Transaction"
```

**Когда использовать:**
- Большинство веб-приложений
- Приложения с короткими транзакциями
- Высокая нагрузка на базу данных

**Плюсы:**
- Высокая эффективность использования подключений
- Хорошая масштабируемость
- Подходит для большинства сценариев

**Минусы:**
- Prepared statements могут не работать между транзакциями
- Требует правильного управления транзакциями

### Statement Mode

Серверное подключение используется только для выполнения одного запроса:

```toml
[pools.maintmode]
pool_mode = "Statement"
```

**Когда использовать:**
- Приложения с очень короткими запросами
- Сценарии с высокой степенью параллелизма
- Readonly запросы

**Плюсы:**
- Максимальная эффективность использования подключений
- Минимальное время удержания подключений

**Минусы:**
- Не подходит для транзакций
- Ограниченная функциональность

## Настройка пользователей

### Конфигурация пользователя

```toml
[[pools.maintmode.users]]
username = "postgres"
password = "md53175bce1d3201d16594cebf9d7eb3f9d"
server_username = "postgres"
server_password = "postgres"
pool_size = 40
```

### Генерация MD5 хеша пароля

```bash
echo -n "postgrespostgres" | md5sum
# Результат: 3175bce1d3201d16594cebf9d7eb3f9d
# Добавьте префикс "md5": md53175bce1d3201d16594cebf9d7eb3f9d
```

### Несколько пользователей

```toml
[[pools.maintmode.users]]
username = "app_user"
password = "md5..."
server_username = "postgres"
server_password = "postgres"
pool_size = 20

[[pools.maintmode.users]]
username = "readonly_user"
password = "md5..."
server_username = "readonly"
server_password = "readonly"
pool_size = 10
```

## Мониторинг через Prometheus

### Доступ к метрикам

```bash
curl http://localhost:9127/metrics
```

### Основные метрики

| Метрика | Описание |
|---------|----------|
| `pg_doorman_connections_total` | Общее количество подключений |
| `pg_doorman_connections_active` | Активные подключения |
| `pg_doorman_connections_idle` | Неактивные подключения |
| `pg_doorman_queries_total` | Общее количество запросов |
| `pg_doorman_queries_duration_seconds` | Длительность запросов |
| `pg_doorman_pool_size` | Размер пула подключений |
| `pg_doorman_pool_available` | Доступные подключения в пуле |

### Настройка Prometheus

```yaml
scrape_configs:
  - job_name: 'pg_doorman'
    static_configs:
      - targets: ['pg_doorman:9127']
```

### Grafana Dashboard

Рекомендуемые панели для Grafana:
1. Активные подключения
2. Неактивные подключения
3. Размер пула
4. Длительность запросов (p50, p95, p99)
5. Ошибки подключений

## Интеграция с приложением

### Настройка DSN для подключения

```go
dsn := "postgres://postgres:postgres@pg_doorman:6432/maintmode?sslmode=disable"
```

### Настройка пула подключений в приложении

```go
config, err := pgxpool.ParseConfig(dsn)
if err != nil {
    return err
}

config.MaxConns = 50
config.MinConns = 20
config.MaxConnLifetime = 10 * time.Minute
config.MaxConnIdleTime = 5 * time.Minute

pool, err := pgxpool.NewWithConfig(ctx, config)
```

### Переменные окружения в docker-compose

```yaml
maintmode:
  environment:
    DB_DSN: postgres://postgres:postgres@pg_doorman:6432/maintmode?sslmode=disable
    DB_DRIVER: postgres
    DB_MAX_OPEN_CONNS: 50
    DB_MAX_IDLE_CONNS: 20
    DB_CONNECTIONS_MAX_LIFETIME: 10m
    DB_CONNECTION_MAX_IDLE_TIME: 5m
```

## Health Checks

### Docker Compose healthcheck

```yaml
pg_doorman:
  image: ghcr.io/ozontech/pg_doorman:latest
  healthcheck:
    test: ["CMD", "pg_isready", "-h", "localhost", "-p", "6432", "-U", "postgres"]
    interval: 10s
    timeout: 5s
    retries: 5
    start_period: 5s
```

### Проверка подключения

```bash
psql -h localhost -p 6432 -U postgres -d maintmode
```

## Оптимизация производительности

### Настройка pool_size

```toml
[[pools.maintmode.users]]
pool_size = 40
```

**Рекомендации:**
- Для небольших приложений: 10-20
- Для средних приложений: 20-50
- Для крупных приложений: 50-100+
- Учитывайте количество воркеров/горутин в приложении

### Настройка worker_threads

```toml
worker_threads = 4
```

**Рекомендации:**
- Установите равным количеству CPU ядер
- Для высоконагруженных систем: 8-16

### Настройка таймаутов

```toml
connect_timeout = 3000
query_wait_timeout = 5000
idle_timeout = 300000000
```

**Рекомендации:**
- `connect_timeout`: 2000-5000 мс
- `query_wait_timeout`: 3000-10000 мс
- `idle_timeout`: 300000000-600000000 нс (5-10 минут)

## Полный пример docker-compose

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

volumes:
  db_data:
```

## Полезные команды

### Просмотр логов pg_doorman

```bash
docker logs -f pg_doorman
```

### Проверка метрик

```bash
curl http://localhost:9127/metrics
```

### Перезапуск pg_doorman

```bash
docker-compose restart pg_doorman
```

### Подключение к PostgreSQL через pg_doorman

```bash
psql -h localhost -p 6432 -U postgres -d maintmode
```

## Рекомендации

1. **Используйте Transaction mode** для большинства веб-приложений
2. **Настройте pool_size** исходя из нагрузки и количества воркеров
3. **Включите Prometheus метрики** для мониторинга производительности
4. **Настройте health checks** для автоматического восстановления
5. **Используйте md5 хеши** для паролей в конфигурации
6. **Мониторьте idle_timeout** для предотвращения утечек подключений
7. **Тестируйте конфигурацию** на staging окружении перед production

## Ресурсы

- [pg_doorman GitHub Repository](https://github.com/ozontech/pg_doorman)
- [pg_doorman Documentation](https://github.com/ozontech/pg_doorman#pg_doorman)
- [PostgreSQL Connection Pooling](https://wiki.postgresql.org/wiki/Connection_pooling)

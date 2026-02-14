# Конфигурация pg_doorman

## Полная конфигурация pg_doorman.toml

```toml
[general]
host = "0.0.0.0"
port = 6432
connect_timeout = 3000
query_wait_timeout = 5000
idle_timeout = 300000000
max_connections = 8192
worker_threads = 4

[prometheus]
host = "0.0.0.0"
port = 9127
enabled = true

[pools.maintmode]
pool_mode = "Transaction"
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

## Раздел [general]

| Параметр | Описание | Значение по умолчанию |
|----------|----------|----------------------|
| `host` | Хост для прослушивания | 0.0.0.0 |
| `port` | Порт для подключений клиентов | 6432 |
| `connect_timeout` | Таймаут подключения (мс) | 3000 |
| `query_wait_timeout` | Таймаут ожидания запроса (мс) | 5000 |
| `idle_timeout` | Таймаут неактивности (нс) | 300000000 |
| `max_connections` | Макс. клиентских подключений | 8192 |
| `worker_threads` | Количество рабочих потоков | 4 |

## Генерация MD5 пароля

```bash
echo -n "postgrespostgres" | md5sum
# Результат: 3175bce1d3201d16594cebf9d7eb3f9d
# Добавьте префикс "md5": md53175bce1d3201d16594cebf9d7eb3f9d
```

## Оптимизация производительности

### pool_size
- Небольшие приложения: 10-20
- Средние приложения: 20-50
- Крупные приложения: 50-100+

### worker_threads
- Установите равным количеству CPU ядер
- Для высоконагруженных систем: 8-16

### Таймауты
- `connect_timeout`: 2000-5000 мс
- `query_wait_timeout`: 3000-10000 мс
- `idle_timeout`: 300000000-600000000 нс

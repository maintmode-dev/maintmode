# Мониторинг pg_doorman

## Prometheus метрики

### Доступ к метрикам

```bash
curl http://localhost:9127/metrics
```

### Основные метрики

#### Подключения
- `pg_doorman_connections_total` - Общее количество
- `pg_doorman_connections_active` - Активные
- `pg_doorman_connections_idle` - Неактивные

#### Пул
- `pg_doorman_pool_size` - Размер пула
- `pg_doorman_pool_available` - Доступные подключения
- `pg_doorman_pool_used` - Используемые подключения

#### Запросы
- `pg_doorman_queries_total` - Общее количество
- `pg_doorman_queries_duration_seconds` - Длительность
- `pg_doorman_queries_failed` - Ошибки

## Настройка Prometheus

```yaml
scrape_configs:
  - job_name: 'pg_doorman'
    static_configs:
      - targets: ['pg_doorman:9127']
```

### Полезные запросы

```promql
# Активные подключения
pg_doorman_connections_active{pool="maintmode"}

# Использование пула (%)
(pg_doorman_pool_used / pg_doorman_pool_size) * 100

# P95 длительность запросов
histogram_quantile(0.95, pg_doorman_queries_duration_seconds_bucket)
```

## Grafana Dashboard

### Рекомендуемые панели

1. **Connection Overview** - активные/неактивные подключения
2. **Pool Utilization** - использование пула (%)
3. **Query Performance** - QPS, P50/P95/P99 duration
4. **Errors** - connection/query failures

## Health Checks

```yaml
healthcheck:
  test: ["CMD", "pg_isready", "-h", "localhost", "-p", "6432"]
  interval: 10s
  timeout: 5s
  retries: 5
```

## Alerting Rules

```yaml
- alert: HighPoolUtilization
  expr: (pg_doorman_pool_used / pg_doorman_pool_size) > 0.9
  for: 5m

- alert: ConnectionErrors
  expr: rate(pg_doorman_connections_failed[5m]) > 0.1
  for: 2m
```

## Troubleshooting

### Высокое использование пула
- Увеличить `pool_size`
- Оптимизировать запросы
- Увеличить `worker_threads`

### Ошибки подключений
- Проверить доступность PostgreSQL
- Проверить credentials
- Увеличить `connect_timeout`

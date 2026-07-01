# Project Overview

# Maintenance Calendar

**Maintenance Calendar** — B2B веб-приложение для инженерных команд, которое помогает **планировать и выполнять технические работы без конфликтов и неожиданных инцидентов**.

Продукт предоставляет единый календарь технических работ, учитывает **временные пересечения и общие ресурсы**, а также разделяет **плановое и фактическое выполнение** работ.

Основная цель — **предотвращение продакшн-проблем до их возникновения**, за счёт прозрачности и координации между командами.

---

## Для кого

* Tech Leads
* SRE / DevOps инженеры
* Platform и Backend команды
* Компании с 10–100 инженерами и общей инфраструктурой

---

## Ключевая идея

> **Время + общие ресурсы = риск**

Если несколько изменений затрагивают один и тот же ресурс в пересекающееся время, это потенциальный инцидент.
Приложение делает такие ситуации **явными и управляемыми**.

---

## Возможности

* Единый календарь технических работ (неделя / месяц)
* Планирование работ с указанием временного окна
* Фиксация фактического времени выполнения
* Учет общих ресурсов (сервисы, базы данных, кластеры)
* Обнаружение временных конфликтов
* Наглядная визуализация рисков
* Уведомления для команд

---

## Архитектурный подход

* Чёткое разделение ответственности между слоями
* PostgreSQL как источник истины
* Работа с временем как с доменной сущностью
* Атомарные операции и корректная работа в конкурентной среде
* Минимум магии, максимум предсказуемости

---

## Tech Stack

### Backend стек

- **Language**: Go 1.25.0+
- **Database**: PostgreSQL

- **HTTP Framework**: Echo
- **Database Libraries**:
  - `jet` - Type-safe SQL builder
  - `sqlx` - Extensions for database/sql
  - `goose` - Database migrations
  * `tstzrange`
  * GiST индексы
* Миграции: `goose` / `migrate`
* Redis (вне ядра): idempotency, rate limiting
- **Logging**: `zap` with `xlog` wrapper (github.com/ruko1202/xlog)

### Deployment

- **Format**: Docker image
---

### Frontend стек

* **React**
* **TypeScript**
* **Vite**
* **MUI (Material UI)**
* **FullCalendar**

UI ориентирован на:

* быстрый обзор планируемых работ
* минимальное количество действий
* понятную визуализацию конфликтов и рисков

---

## Интеграции

* Slack (уведомления и действия)
* Email (опционально)

---

## Философия проекта

* Простота важнее универсальности
* Предсказуемость важнее магии
* Предотвращение важнее реакции
* Инженерная честность важнее количества фич

---

Если хочешь, следующим шагом можем:

* сократить README под **лендинг**
* оформить **архитектурную диаграмму**
* подготовить **onboarding для первых пользователей**





## Operations

Deploy orchestration (single-VM Docker Compose, HTTPS via Caddy, monitoring,
rolling deploy) lives in the separate **[`maintmode-deploy`](../maintmode-deploy/README.md)**
repository — it pulls the images this repo's CI publishes to GHCR. This repo
keeps only what's needed for local dev and CI (`compose.yaml`, the dev Caddy
image/config, app configs). Operator runbooks, backup procedures, and the
smoke-test checklist are in **maintmode-docs/ops/**
([`maintmode-docs`](../maintmode-docs/ops/README.md)).

### Scaling

`maintmode` and `auth` are stateless and can run as N replicas on the
same VM. `make app-up` accepts replica counts (default 1/1):

```bash
make app-up MAINTMODE_REPLICAS=3 AUTH_REPLICAS=3
# or directly:
docker compose --profile storages --profile app --profile monitoring \
  up -d --scale maintmode=3 --scale auth=3
```

How it works:

- Docker assigns each replica a unique name (`*-maintmode-1`,
  `*-maintmode-2`, ...). The service-name DNS (`maintmode`, `auth`)
  resolves to all replica IPs.
- Caddy load-balances across replicas (`lb_policy round_robin`) with
  passive health checks and request retries on a peer.
- The Goque task queue uses `FOR UPDATE SKIP LOCKED`, so the same task
  is never picked up by two replicas.
- OAuth login rate-limiting is shared via Redis with a per-replica
  in-memory fallback if Redis is unreachable (alert
  `RateLimiterRedisFallback`).
- Prometheus discovers every replica via Docker SD; Loki/Promtail
  labels logs by `container_name` so replica logs are easy to filter.

Deploys are rolling and zero-downtime: the deploy repo's `make deploy` rolls
the new image through one replica at a time, leaning on the in-app drain
(Readiness → 503 on SIGTERM) and Caddy's active `/readiness` health check so
the pool never drops below its healthy count. See
[rolling-deploy.md](../maintmode-docs/ops/rolling-deploy.md).

Known limitations on a single VM:

- Postgres, Redis, and Caddy stay single-instance — true HA needs
  external infra (multi-node HA is the P2 Kubernetes milestone, not this
  target).
- After changing the replica count, restart Caddy
  (`docker compose restart caddy`) so it re-resolves DNS.

### Quick Start

```bash
# 1. Install dependencies
make deps

# 2. Install binary tools
make bin-deps

# 3. Build application
make build

# 4. Run application
./bin/maintmode
```

### Configuration and Secrets

Application config and secrets are split intentionally:

- `app.*.yaml` contains non-secret settings and secret references such as `<secret:db/dsn>`.
- `secrets.yaml` is a flat key-value file mounted at runtime and ignored by git.
- Tracked `secrets.*.sample.yaml` files are only for local/dev/ci bootstrapping.

Each service reads `app.config.yaml` and `app.secrets.yaml` from
`<APP>_CONFIG_DIR`, and `model.conf` + `policy.csv` from
`<APP>_AUTHZ_DIR`. File names can be overridden via
`<APP>_CONFIG_FILE` / `<APP>_SECRETS_FILE`. All env vars default to
`.` (cwd) when unset.

```bash
MAINTMODE_CONFIG_DIR=deployment/maintmode/prod
MAINTMODE_AUTHZ_DIR=deployment/maintmode/authz
```

Production deploys should keep real values in the cloud secret manager, then mount or generate a read-only `/app.secrets.yaml` for each container. The app does not call cloud-specific secret APIs.

### Development Commands

```bash
# Run lightweight secret checks
make secret-scan

# Run tests with coverage
make tloc-cov

# Run linter
make lint

# Format code
make fmt

# Build application
make build

# Start database (Docker)
make docker-up

# Start the app without build
make run

# Start the app with live reload
# allowed debug connection in IDE
###########################
# GoLand
#   Run → Edit Configurations
#   Go Remote
#   Host: localhost
#   Port: 2345
###########################
# Vscode
# {
#  "version": "0.2.0",
#  "configurations": [
#    {
#      "name": "Attach to Delve (Air)",
#      "type": "go",
#      "request": "attach",
#      "mode": "remote",
#      "remotePath": "${workspaceFolder}",
#      "port": 2345,
#      "host": "127.0.0.1"
#    }
#  ]
# }
 
make air
```

## Компоненты мониторинга
| Сервис | URL | Логин/Пароль |
|--------|-----|--------------|
| Grafana | http://localhost:8003 | admin/admin |
| VictoriaMetrics | http://localhost:8428 | - |
| Loki | http://localhost:3100 | - |

### VictoriaMetrics
- **Порт**: 8428
- **Назначение**: Time Series Database для метрик
- **Retention**: 30 дней
- **Конфигурация**: [`monitoring/config/prometheus.yml`](config-monitoring-prometheus.yml.md)

### Grafana
- **Порт**: 3000
- **Назначение**: Визуализация метрик и логов
- **Datasources**: VictoriaMetrics, Loki
- **Дашборды**: Автоматическая загрузка из provisioning

### Loki
- **Порт**: 3100
- **Назначение**: Агрегация логов
- **Retention**: 30 дней
- **Конфигурация**: [`monitoring/config/loki/local-config.yaml`](config-monitoring-loki-local-config.yaml.md)

### Promtail
- **Назначение**: Сбор логов из Docker контейнеров
- **Конфигурация**: [`monitoring/config/promtail/config.yml`](config-monitoring-promtail-config.yml.md)

### Pyroscope (continuous profiling, RUK-190)
- **Порт**: 4040 (внутренний, наружу не публикуется)
- **Назначение**: Непрерывное профилирование приложения (CPU, heap, allocations,
  goroutines, block, mutex)
- **Модель**: pull — Grafana **Alloy** скрейпит стандартные Go `pprof`-эндпоинты
  приложения (на infra-сервере `:8001`) и форвардит профили в Pyroscope. Приложение
  ничего не пушит; SDK не подключается.
- **Масштабирование**: Alloy находит цели через `discovery.docker` по лейблу
  `prometheus.service=maintmode`, поэтому `docker compose up --scale maintmode=N`
  профилируется автоматически (по цели на реплику).
- **Хранение**: named volume `pyroscope:/data`. Ретеншен двумя рычагами:
  по времени — блоки старше 2 суток (48h, рядом с Tempo 24h); по диску —
  страховка от разрастания: чистка старых блоков, если на хосте свободно < 10 ГБ.
- **Просмотр**: Grafana → datasource **Pyroscope** (Explore) или дашборд
  **MaintMode Profiling**; ищите по тегу `service_name="maintmode"`.
- **Конфигурация**: [`monitoring/config/alloy/config.alloy`](config-monitoring-alloy-config.alloy.md)
- **Версии**: `grafana/pyroscope:1.14.1` (стабильная all-in-one линия; 2.x требует
  multi-target деплой), `grafana/alloy:v1.17.1`.

### Экспортеры
- **Node Exporter** (9100) - Метрики хост-системы
- **cAdvisor** (8080) - Метрики Docker контейнеров
- **PostgreSQL Exporter** (9187) - Метрики PostgreSQL
- **Redis Exporter** (9121) - Метрики Redis

## Дашборды

| Дашборд | Описание | Источник |
|---------|-----------|----------|
| MaintMode Application | HTTP метрики, бизнес-метрики, Go runtime | Custom |
| PostgreSQL Database | Производительность PostgreSQL | Marketplace (ID: 9628) |
| Redis Dashboard | Производительность Redis | Marketplace (ID: 11835) |
| PgBouncer Exporter | Метрики pg_doorman | Marketplace (ID: 11271) |
| cAdvisor | Метрики Docker контейнеров | Marketplace (ID: 893) |
| Node Exporter Full | Метрики хост-системы | Marketplace (ID: 1860) |
| MaintMode Profiling | Флеймграфы CPU/heap/alloc/goroutine/block/mutex (Pyroscope) | Custom |

## Метрики приложения

### HTTP метрики (echoprometheus)
- `echo_http_requests_total{method, path, status}` - Общее количество HTTP запросов
- `echo_http_request_duration_seconds_bucket{method, path, le}` - Histogram latency
- `echo_http_requests_in_flight{method, path}` - Текущие запросы

### Go runtime
- `go_goroutines` - Количество goroutines
- `go_gc_duration_seconds_sum` - Суммарное время GC
- `go_gc_duration_seconds_count` - Количество GC циклов


## Полезные ссылки

- [VictoriaMetrics Documentation](https://docs.victoriametrics.com/)
- [Grafana Documentation](https://grafana.com/docs/)
- [Loki Documentation](https://grafana.com/docs/loki/latest/)
- [Grafana Marketplace](https://grafana.com/grafana/dashboards/)
- [Prometheus Best Practices](https://prometheus.io/docs/practices/naming/)

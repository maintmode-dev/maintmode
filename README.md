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

For deploying MaintMode to a single VM with HTTPS, monitoring, backup
runbooks, and the production smoke-test checklist, see the
**maintmode-docs/ops/** directory in the separate
[`maintmode-docs`](../maintmode-docs/ops/README.md) repository. The
backend repo holds the compose files and Caddy configuration; the
docs repo is the canonical source for operator procedures.

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

AUTH_CONFIG_DIR=deployment/auth/prod
AUTH_AUTHZ_DIR=deployment/auth/authz
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

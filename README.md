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

### Development Commands

```bash
# Run tests with coverage
make test-cov

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


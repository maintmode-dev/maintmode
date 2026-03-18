# K6 Load Tests - Quick Start

## 1. Как запустить

### Запустить окружение с инфраструктурой

```bash
# Запустить приложение + мониторинг (Grafana, Victoria Metrics)
make app-with-monitoring-up
```

### Запустить нагрузочный тест

```bash
# Запустить полный сценарий (рекомендуется для первого раза)
make k6
```

Тест автоматически:
- Подключится к API на `http://maintmode:8000`
- Отправит метрики в Victoria Metrics
- Запустит имитацию работы команды инженеров (~5 минут)

## 2. Как настроить нагрузочные тесты

### Изменить профиль нагрузки

Откройте нужный тест в `test/k6/scenarios/` и измените `options.stages`:

```javascript
export const options = {
  stages: [
    { duration: '30s', target: 5 },   // Плавный рост до 5 VUs за 30 секунд
    { duration: '1m', target: 10 },   // Рост до 10 VUs за 1 минуту
    { duration: '2m', target: 10 },   // Удержание 10 VUs в течение 2 минут
    { duration: '30s', target: 0 },   // Плавное снижение до 0 за 30 секунд
  ],
};
```

**Параметры:**
- `duration` - длительность этапа (`30s`, `1m`, `2m`)
- `target` - целевое количество виртуальных пользователей (VUs)

### Изменить пороги успешности (thresholds)

```javascript
export const options = {
  thresholds: {
    http_req_duration: ['p(95)<500', 'p(99)<1000'], // 95% запросов < 500ms
    http_req_failed: ['rate<0.05'],                  // Ошибок < 5%
    checks: ['rate>0.95'],                           // Проверок успешно > 95%
  },
};
```

### Настроить паузы между запросами

```javascript
import { sleep } from 'k6';

export default function() {
  // Ваш тест

  sleep(1);  // Пауза 1 секунда между итерациями
  // или
  sleep(Math.random() * 3); // Случайная пауза 0-3 секунды
}
```

### Примеры профилей нагрузки

**Легкий тест (smoke test):**
```javascript
stages: [
  { duration: '1m', target: 1 },  // 1 пользователь
]
```

**Средняя нагрузка:**
```javascript
stages: [
  { duration: '1m', target: 10 },
  { duration: '3m', target: 10 },
  { duration: '1m', target: 0 },
]
```

**Стресс-тест:**
```javascript
stages: [
  { duration: '2m', target: 50 },
  { duration: '5m', target: 50 },
  { duration: '2m', target: 100 },  // Резкий скачок
  { duration: '3m', target: 100 },
  { duration: '2m', target: 0 },
]
```

**Spike test (пиковая нагрузка):**
```javascript
stages: [
  { duration: '10s', target: 100 },  // Резкий скачок
  { duration: '1m', target: 100 },
  { duration: '10s', target: 0 },
]
```

## 3. Где смотреть результаты

### В Grafana (визуализация в реальном времени)

```bash
# Открыть Grafana dashboard
open http://localhost:8003/d/k6-load-testing
```

**Логин:** `admin` / **Пароль:** `admin`

**Dashboard показывает:**
- 📊 **Performance Overview** - VUs, время ответа, request rate, ошибки
- ⏱️ **HTTP Latency Timings** - разбивка времени запроса (connecting, waiting, receiving)
- 📈 **HTTP Request Rate** - throughput (запросов в секунду)
- 📋 **Requests by URL** - статистика по каждому endpoint (p50/p95/p99)
- ✅ **Checks** - успешность бизнес-проверок
- 📦 **Data Transfer** - объем переданных данных

### В консоли (после завершения теста)

k6 выводит итоговую статистику:

```
     ✓ Create Resource: status is 200
     ✓ Search Resources: response time < 500ms

     checks.........................: 98.50% ✓ 1970    ✗ 30
     data_received..................: 2.1 MB 7.0 kB/s
     http_req_duration..............: avg=145ms  p(95)=390ms p(99)=780ms
     http_req_failed................: 2.10%  ✓ 42      ✗ 1958
     http_reqs......................: 2000   15.07/s
     vus............................: 20     min=1     max=24
```

**Ключевые метрики:**
- ✅ `checks` > 95% - бизнес-логика работает корректно
- ⚡ `http_req_duration` p(95) < 500ms - производительность в норме
- ❌ `http_req_failed` < 5% - низкий уровень ошибок
- 🔄 `http_reqs` - общий throughput

### Критерии успешности

| Метрика | 🟢 Отлично | 🟡 Приемлемо | 🔴 Плохо |
|---------|-----------|-------------|---------|
| **Checks** | > 95% | 90-95% | < 90% |
| **p95 response time** | < 300ms | 300-500ms | > 500ms |
| **Error rate** | < 1% | 1-5% | > 5% |
| **p99 response time** | < 500ms | 500-1000ms | > 1000ms |

## Troubleshooting

### Приложение не отвечает

```bash
# Проверить статус контейнеров
docker ps

# Проверить логи
docker logs maintmode
```

### Метрики не появляются в Grafana

```bash
# Проверить Victoria Metrics
curl http://localhost:8428/-/healthy

# Проверить метрики k6
curl 'http://localhost:8428/api/v1/query?query=k6_vus'
```

### Много ошибок в тестах

1. Проверьте логи API: `make app-logs`
2. Уменьшите нагрузку (меньше VUs)
3. Увеличьте паузы между запросами


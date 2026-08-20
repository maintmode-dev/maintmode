# K6 Load Tests - Quick Start

## 1. How to run

### Bring up the environment with the infrastructure

```bash
# Start the application + monitoring (Grafana, Victoria Metrics)
make app-with-monitoring-up
```

### Get an authorization token

The API requires a Bearer token. Signup is invite-only by default (a plain
exchange through the stub yields a rejection, a guest or — on an empty database
only — the bootstrap admin), so the token is minted through the OAuth stub
**with the dev-only `X-Test-Roles` header** — it creates a stub user and grants
the roles the scenarios need (admin covers every write). The token has a limited
TTL — for long soak runs, re-mint it right before starting:

```bash
AUTH_TOKEN=$(curl -s -X POST "http://localhost:9000/auth/api/v1/login/oauth/exchange/google" \
  -H 'Content-Type: application/json' -H 'X-Test-Roles: admin' \
  -d '{"id_token":"k6-load-test"}' | jq -r '.access_token')
```

The token is passed into the scenarios through the environment:

```bash
AUTH_TOKEN=$AUTH_TOKEN make k6
# or directly: k6 run -e AUTH_TOKEN=$AUTH_TOKEN scenarios/00-full-scenario-test.js
```

### Run a load test

```bash
# Run the full scenario (recommended for the first run)
make k6
```

The test automatically:
- Connects to the API at `http://maintmode:8000`
- Sends metrics to Victoria Metrics
- Simulates the work of an engineering team (~5 minutes)

## 2. How to configure the load tests

### Change the load profile

Open the test you need in `test/k6/scenarios/` and edit `options.stages`:

```javascript
export const options = {
  stages: [
    { duration: '30s', target: 5 },   // Ramp up smoothly to 5 VUs over 30 seconds
    { duration: '1m', target: 10 },   // Ramp up to 10 VUs over 1 minute
    { duration: '2m', target: 10 },   // Hold 10 VUs for 2 minutes
    { duration: '30s', target: 0 },   // Ramp down smoothly to 0 over 30 seconds
  ],
};
```

**Parameters:**
- `duration` - the length of the stage (`30s`, `1m`, `2m`)
- `target` - the target number of virtual users (VUs)

### Change the success thresholds

```javascript
export const options = {
  thresholds: {
    http_req_duration: ['p(95)<500', 'p(99)<1000'], // 95% of requests < 500ms
    http_req_failed: ['rate<0.05'],                  // Errors < 5%
    checks: ['rate>0.95'],                           // Checks passing > 95%
  },
};
```

### Tune the pauses between requests

```javascript
import { sleep } from 'k6';

export default function() {
  // Your test

  sleep(1);  // A 1 second pause between iterations
  // or
  sleep(Math.random() * 3); // A random pause of 0-3 seconds
}
```

### Example load profiles

**Light test (smoke test):**
```javascript
stages: [
  { duration: '1m', target: 1 },  // 1 user
]
```

**Moderate load:**
```javascript
stages: [
  { duration: '1m', target: 10 },
  { duration: '3m', target: 10 },
  { duration: '1m', target: 0 },
]
```

**Stress test:**
```javascript
stages: [
  { duration: '2m', target: 50 },
  { duration: '5m', target: 50 },
  { duration: '2m', target: 100 },  // A sharp spike
  { duration: '3m', target: 100 },
  { duration: '2m', target: 0 },
]
```

**Spike test (peak load):**
```javascript
stages: [
  { duration: '10s', target: 100 },  // A sharp spike
  { duration: '1m', target: 100 },
  { duration: '10s', target: 0 },
]
```

## 3. Where to look at the results

### In Grafana (real-time visualization)

```bash
# Open the Grafana dashboard
open http://localhost:8003/d/k6-load-testing
```

**Login:** `admin` / **Password:** `admin`

**The dashboard shows:**
- 📊 **Performance Overview** - VUs, response time, request rate, errors
- ⏱️ **HTTP Latency Timings** - a breakdown of the request time (connecting, waiting, receiving)
- 📈 **HTTP Request Rate** - throughput (requests per second)
- 📋 **Requests by URL** - statistics per endpoint (p50/p95/p99)
- ✅ **Checks** - the pass rate of the business checks
- 📦 **Data Transfer** - the volume of data transferred

### In the console (after the test finishes)

k6 prints the final statistics:

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

**Key metrics:**
- ✅ `checks` > 95% - the business logic works correctly
- ⚡ `http_req_duration` p(95) < 500ms - performance is within the norm
- ❌ `http_req_failed` < 5% - a low error rate
- 🔄 `http_reqs` - the overall throughput

### Success criteria

| Metric | 🟢 Excellent | 🟡 Acceptable | 🔴 Poor |
|---------|-----------|-------------|---------|
| **Checks** | > 95% | 90-95% | < 90% |
| **p95 response time** | < 300ms | 300-500ms | > 500ms |
| **Error rate** | < 1% | 1-5% | > 5% |
| **p99 response time** | < 500ms | 500-1000ms | > 1000ms |

## Troubleshooting

### The application does not respond

```bash
# Check the container status
docker ps

# Check the logs
docker logs maintmode
```

### Metrics do not show up in Grafana

```bash
# Check Victoria Metrics
curl http://localhost:8428/-/healthy

# Check the k6 metrics
curl 'http://localhost:8428/api/v1/query?query=k6_vus'
```

### Too many errors in the tests

1. Check the API logs: `make app-logs`
2. Reduce the load (fewer VUs)
3. Increase the pauses between requests

